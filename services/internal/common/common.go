package common

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func OpenDB() (*sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(6)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func ExecStatements(ctx context.Context, db *sql.DB, statements ...string) error {
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

type Migration struct {
	Version    int
	Name       string
	Statements []string
}

// ApplyMigrations runs ordered, service-scoped PostgreSQL migrations under an
// advisory lock. Each migration is transactional and recorded exactly once.
// This keeps independently deployed microservices safe during rolling deploys
// while preserving the containerized service boundaries.
func ApplyMigrations(ctx context.Context, db *sql.DB, service string, migrations []Migration) error {
	service = strings.TrimSpace(service)
	if service == "" {
		return errors.New("migration service name is required")
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	const registryLock = "himate-migration-registry"
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, registryLock); err != nil {
		return fmt.Errorf("migration registry lock: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS public.himate_schema_migrations(
			service TEXT NOT NULL,
			version INT NOT NULL,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY(service, version)
		)`); err != nil {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, registryLock)
		return fmt.Errorf("migration registry: %w", err)
	}
	if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, registryLock); err != nil {
		return fmt.Errorf("migration registry unlock: %w", err)
	}

	serviceLock := "himate-migration:" + service
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, serviceLock); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, serviceLock)

	current := 0
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM public.himate_schema_migrations WHERE service=$1`, service).Scan(&current); err != nil {
		return fmt.Errorf("migration current version: %w", err)
	}

	lastDeclared := 0
	for _, migration := range migrations {
		if migration.Version <= lastDeclared {
			return fmt.Errorf("migrations for %s must be strictly increasing", service)
		}
		lastDeclared = migration.Version
		if migration.Version <= current {
			continue
		}

		tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return fmt.Errorf("migration %d begin: %w", migration.Version, err)
		}
		for _, stmt := range migration.Statements {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err = tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s/%d %s: %w", service, migration.Version, migration.Name, err)
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO public.himate_schema_migrations(service,version,name) VALUES($1,$2,$3)`, service, migration.Version, migration.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s/%d registry: %w", service, migration.Version, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("migration %s/%d commit: %w", service, migration.Version, err)
		}
		current = migration.Version
	}
	return nil
}

func platformSecretKey(master string) ([32]byte, error) {
	if len(strings.TrimSpace(master)) < 24 {
		return [32]byte{}, errors.New("platform secret master credential is not configured")
	}
	return sha256.Sum256([]byte("himate-platform-secrets-v1\x00" + master)), nil
}

func EncryptPlatformSecret(master, plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("secret value is required")
	}
	key, err := platformSecretKey(master)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte("himate-platform-secret"))
	payload := append(nonce, sealed...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func DecryptPlatformSecret(master, encoded string) (string, error) {
	key, err := platformSecretKey(master)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", errors.New("platform secret payload is invalid")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) <= gcm.NonceSize() {
		return "", errors.New("platform secret payload is truncated")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte("himate-platform-secret"))
	if err != nil {
		return "", errors.New("platform secret could not be decrypted")
	}
	return string(plain), nil
}

func LoadPlatformSecret(ctx context.Context, db *sql.DB, master, secretKey string) (string, bool, error) {
	if db == nil {
		return "", false, errors.New("database is unavailable")
	}
	var encrypted string
	err := db.QueryRowContext(ctx, `SELECT encrypted_value FROM identity.platform_secrets WHERE secret_key=$1 AND active=TRUE`, strings.TrimSpace(secretKey)).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	value, err := DecryptPlatformSecret(master, encrypted)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func MarshalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func JSONRawOrEmpty(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func APIError(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func Decode(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func InternalAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if len(token) < 24 {
			APIError(w, http.StatusServiceUnavailable, "SERVICE_CREDENTIAL_UNAVAILABLE", "Internal service authentication is not configured")
			return
		}
		got := r.Header.Get("X-Himate-Internal-Token")
		if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			APIError(w, http.StatusForbidden, "FORBIDDEN", "Invalid service credential")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func Logged(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func Run(log *slog.Logger, service, port string, handler http.Handler) {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           Logged(log, handler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       25 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Info("service started", "service", service, "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
