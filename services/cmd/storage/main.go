package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const maxObjectBytes = 25 << 20

type app struct {
	db   *sql.DB
	root string
}

var safePartnerID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var safeObjectKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,239}$`)

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()

	root := strings.TrimSpace(os.Getenv("HIMATE_STORAGE_ROOT"))
	if root == "" { root = "/data/partners" }
	if err := os.MkdirAll(root, 0700); err != nil { log.Error("storage root", "error", err); os.Exit(1) }
	a := &app{db: db, root: root}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil { log.Error("migration", "error", err); os.Exit(1) }

	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/internal/v1/storage/partners/", a.partnerRoute)
	mux.HandleFunc("/internal/v1/storage/objects/", a.objectRoute)
	mux.HandleFunc("/internal/v1/storage/summary", a.summary)
	common.Run(log, "storage", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "storage", []common.Migration{
		{Version: 1, Name: "partner-storage", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS storage`,
			`CREATE TABLE IF NOT EXISTS storage.partner_namespaces(
				partner_id TEXT PRIMARY KEY,
				namespace_path TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'READY',
				last_error TEXT NOT NULL DEFAULT '',
				checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		}},
	})
}

func (a *app) pathFor(namespace string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	if !safePartnerID.MatchString(namespace) { return "", fmt.Errorf("invalid storage namespace") }
	return filepath.Join(a.root, namespace), nil
}

func (a *app) objectPath(namespace, key string) (string, error) {
	root, err := a.pathFor(namespace)
	if err != nil { return "", err }
	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if !safeObjectKey.MatchString(key) || strings.Contains(key, "..") || strings.HasSuffix(key, "/") {
		return "", fmt.Errorf("invalid object key")
	}
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
	prefix := filepath.Clean(root) + string(os.PathSeparator)
	if target != filepath.Clean(root) && !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("object key escapes namespace")
	}
	return target, nil
}

func (a *app) ensure(ctx context.Context, partnerID string) (map[string]any, error) {
	path, err := a.pathFor(partnerID)
	if err != nil { return nil, err }
	if err := os.MkdirAll(path, 0700); err != nil { return nil, err }
	if err := os.Chmod(path, 0700); err != nil { return nil, err }
	marker := filepath.Join(path, ".himate-storage")
	if err := os.WriteFile(marker, []byte(partnerID+"\n"), 0600); err != nil { return nil, err }
	_, err = a.db.ExecContext(ctx, `INSERT INTO storage.partner_namespaces(partner_id,namespace_path,status,last_error,checked_at)
		VALUES($1,$2,'READY','',NOW())
		ON CONFLICT(partner_id) DO UPDATE SET namespace_path=EXCLUDED.namespace_path,status='READY',last_error='',checked_at=NOW(),updated_at=NOW()`,
		partnerID, path)
	if err != nil { return nil, err }
	return map[string]any{"partner_id": partnerID, "namespace_path": path, "status": "READY"}, nil
}

func (a *app) check(ctx context.Context, partnerID string) (map[string]any, error) {
	path, err := a.pathFor(partnerID)
	if err != nil { return nil, err }
	marker := filepath.Join(path, ".himate-storage")
	raw, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(raw)) != partnerID {
		msg := "storage marker missing or mismatched"
		if err != nil { msg = err.Error() }
		_, _ = a.db.ExecContext(ctx, `UPDATE storage.partner_namespaces SET status='ERROR',last_error=$2,checked_at=NOW(),updated_at=NOW() WHERE partner_id=$1`, partnerID, msg)
		return map[string]any{"partner_id": partnerID, "status": "ERROR", "error": msg}, fmt.Errorf("%s", msg)
	}
	probe := filepath.Join(path, ".health-"+fmt.Sprint(time.Now().UnixNano()))
	if err := os.WriteFile(probe, []byte("ok"), 0600); err != nil { return nil, err }
	if err := os.Remove(probe); err != nil { return nil, err }
	_, _ = a.db.ExecContext(ctx, `UPDATE storage.partner_namespaces SET status='READY',last_error='',checked_at=NOW(),updated_at=NOW() WHERE partner_id=$1`, partnerID)
	return map[string]any{"partner_id": partnerID, "status": "READY", "checked_at": time.Now().UTC()}, nil
}

func (a *app) partnerRoute(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/storage/partners/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[0] == "" { common.APIError(w, 404, "NOT_FOUND", "Storage route not found"); return }
	partnerID, action := parts[0], parts[1]
	switch {
	case r.Method == http.MethodPost && action == "ensure":
		out, err := a.ensure(r.Context(), partnerID)
		if err != nil { common.APIError(w, 500, "STORAGE", err.Error()); return }
		common.JSON(w, 200, out)
	case r.Method == http.MethodGet && action == "health":
		out, err := a.check(r.Context(), partnerID)
		if err != nil { common.JSON(w, 503, out); return }
		common.JSON(w, 200, out)
	default:
		common.APIError(w, 405, "METHOD", "Use POST /ensure or GET /health")
	}
}

func (a *app) objectRoute(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/storage/objects/"), "/")
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		common.APIError(w, 404, "NOT_FOUND", "Storage object route not found")
		return
	}
	namespace, key := parts[0], parts[1]
	target, err := a.objectPath(namespace, key)
	if err != nil { common.APIError(w, 400, "VALIDATION", err.Error()); return }

	switch r.Method {
	case http.MethodPut:
		if _, err := a.ensure(r.Context(), namespace); err != nil { common.APIError(w,500,"STORAGE",err.Error());return }
		if err := os.MkdirAll(filepath.Dir(target),0700); err != nil { common.APIError(w,500,"STORAGE","Could not prepare object directory");return }
		tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
		if err != nil { common.APIError(w,500,"STORAGE","Could not prepare object");return }
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(tmp,h), io.LimitReader(r.Body,maxObjectBytes+1))
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil { common.APIError(w,500,"STORAGE","Could not persist object");return }
		if n > maxObjectBytes { common.APIError(w,413,"OBJECT_TOO_LARGE","Object exceeds 25 MiB");return }
		if err := os.Chmod(tmpName,0600); err != nil { common.APIError(w,500,"STORAGE","Could not secure object");return }
		if err := os.Rename(tmpName,target); err != nil { common.APIError(w,500,"STORAGE","Could not finalize object");return }
		sum := hex.EncodeToString(h.Sum(nil))
		common.JSON(w,200,map[string]any{"namespace":namespace,"object_key":key,"size_bytes":n,"sha256":sum})
	case http.MethodGet:
		file, err := os.Open(target)
		if os.IsNotExist(err) { common.APIError(w,404,"NOT_FOUND","Object not found");return }
		if err != nil { common.APIError(w,500,"STORAGE","Could not open object");return }
		defer file.Close()
		info, err := file.Stat()
		if err != nil { common.APIError(w,500,"STORAGE","Could not stat object");return }
		if contentType := strings.TrimSpace(r.URL.Query().Get("content_type")); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		} else {
			w.Header().Set("Content-Type","application/octet-stream")
		}
		w.Header().Set("Content-Length",fmt.Sprint(info.Size()))
		w.Header().Set("Cache-Control","private, no-store")
		_, _ = io.Copy(w,file)
	case http.MethodDelete:
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) { common.APIError(w,500,"STORAGE","Could not delete object");return }
		w.WriteHeader(http.StatusNoContent)
	default:
		common.APIError(w,405,"METHOD","Use PUT, GET or DELETE")
	}
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	probe := filepath.Join(a.root, ".service-health")
	if err := os.WriteFile(probe, []byte("ok"), 0600); err != nil {
		common.JSON(w, 503, map[string]any{"status": "error", "service": "storage", "error": err.Error()}); return
	}
	_ = os.Remove(probe)
	common.JSON(w, 200, map[string]any{"status": "ok", "service": "storage", "time": time.Now().UTC()})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	rows, err := a.db.Query(`SELECT partner_id FROM storage.partner_namespaces ORDER BY partner_id`)
	if err != nil { common.APIError(w, 500, "DB", "Could not load storage summary"); return }
	defer rows.Close()
	partnerIDs := []string{}
	for rows.Next() { var id string; if rows.Scan(&id)==nil { partnerIDs=append(partnerIDs,id) } }
	items := []map[string]any{}
	for _,partnerID := range partnerIDs {
		out,checkErr := a.check(r.Context(),partnerID)
		if checkErr != nil { items=append(items,out); continue }
		items=append(items,out)
	}
	common.JSON(w, 200, map[string]any{"items": items})
}
