package main

import (
	"context"
	"database/sql"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type app struct{ db *sql.DB }

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil { log.Error("migration", "error", err); os.Exit(1) }

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "partner-runtime", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/internal/v1/runtime/deploy", a.deploy)
	mux.HandleFunc("/internal/v1/runtime/health", a.runtimeHealth)
	mux.HandleFunc("/internal/v1/runtime/summary", a.summary)
	common.Run(log, "partner-runtime", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "runtime", []common.Migration{
		{Version: 1, Name: "partner-runtime", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS runtime`,
			`CREATE TABLE IF NOT EXISTS runtime.deployments(
				partner_id TEXT NOT NULL,
				environment TEXT NOT NULL,
				hostname TEXT NOT NULL UNIQUE,
				release TEXT NOT NULL,
				config JSONB NOT NULL DEFAULT '{}'::jsonb,
				status TEXT NOT NULL DEFAULT 'READY',
				deployed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,environment)
			)`,
		}},
	})
}

func validEnvironment(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v != "STAGING" && v != "PRODUCTION" { return "" }
	return v
}

func (a *app) deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	var in struct {
		PartnerID   string         `json:"partner_id"`
		Environment string         `json:"environment"`
		Hostname    string         `json:"hostname"`
		Release     string         `json:"release"`
		Config      map[string]any `json:"config"`
	}
	if common.Decode(r, &in) != nil || strings.TrimSpace(in.PartnerID) == "" || strings.TrimSpace(in.Hostname) == "" {
		common.APIError(w, 400, "VALIDATION", "partner_id and hostname are required"); return
	}
	in.Environment = validEnvironment(in.Environment)
	if in.Environment == "" { common.APIError(w, 400, "VALIDATION", "STAGING or PRODUCTION environment is required"); return }
	if strings.TrimSpace(in.Release) == "" { in.Release = "current" }
	raw, err := common.MarshalJSON(in.Config)
	if err != nil { common.APIError(w, 400, "VALIDATION", "Invalid runtime config"); return }
	_, err = a.db.Exec(`INSERT INTO runtime.deployments(partner_id,environment,hostname,release,config,status)
		VALUES($1,$2,$3,$4,$5::jsonb,'READY')
		ON CONFLICT(partner_id,environment) DO UPDATE SET hostname=EXCLUDED.hostname,release=EXCLUDED.release,config=EXCLUDED.config,status='READY',deployed_at=NOW(),updated_at=NOW()`,
		strings.TrimSpace(in.PartnerID), in.Environment, strings.ToLower(strings.TrimSpace(in.Hostname)), strings.TrimSpace(in.Release), string(raw))
	if err != nil { common.APIError(w, 409, "CONFLICT", "Runtime deployment or hostname conflicts with an existing assignment"); return }
	common.JSON(w, 200, map[string]any{"partner_id": in.PartnerID, "environment": in.Environment, "hostname": strings.ToLower(strings.TrimSpace(in.Hostname)), "release": strings.TrimSpace(in.Release), "status": "READY"})
}

func (a *app) runtimeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	environment := validEnvironment(r.URL.Query().Get("environment"))
	var hostname, release, status string
	var deployed time.Time
	err := a.db.QueryRow(`SELECT hostname,release,status,deployed_at FROM runtime.deployments WHERE partner_id=$1 AND environment=$2`, partnerID, environment).
		Scan(&hostname, &release, &status, &deployed)
	if err != nil { common.APIError(w, 404, "NOT_DEPLOYED", "Partner runtime is not deployed"); return }
	code := http.StatusOK
	if status != "READY" { code = http.StatusServiceUnavailable }
	common.JSON(w, code, map[string]any{"partner_id": partnerID, "environment": environment, "hostname": hostname, "release": release, "status": status, "deployed_at": deployed, "checked_at": time.Now().UTC()})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	rows, err := a.db.Query(`SELECT partner_id,environment,hostname,release,status,deployed_at,updated_at FROM runtime.deployments ORDER BY partner_id,environment`)
	if err != nil { common.APIError(w, 500, "DB", "Could not load runtime summary"); return }
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var partnerID, environment, hostname, release, status string
		var deployed, updated time.Time
		if rows.Scan(&partnerID, &environment, &hostname, &release, &status, &deployed, &updated) == nil {
			items = append(items, map[string]any{"partner_id": partnerID, "environment": environment, "hostname": hostname, "release": release, "status": status, "deployed_at": deployed, "updated_at": updated})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}
