package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"himate.local/services/internal/common"
	"html"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const sessionCookie = "himate_session"
const passwordIterations = 210000

type app struct {
	db               *sql.DB
	secret           string
	internalToken    string
	webDir           string
	env              string
	version          string
	ttl              time.Duration
	rememberTTL      time.Duration
	secureCookie     bool
	client           *http.Client
	proxies          map[string]*httputil.ReverseProxy
	hosts            map[string]string
	dashboardMu      sync.RWMutex
	dashboardPayload map[string]any
	dashboardExpires time.Time
	loginMu          sync.Mutex
	loginAttempts    map[string]loginState
	auditQueue       chan auditEvent
}

type loginState struct {
	Failures     int
	WindowStart  time.Time
	BlockedUntil time.Time
}

type auditEvent struct {
	ActorID       string
	ActorName     string
	ActorRoles    []string
	RequestID     string
	CorrelationID string
	Action        string
	Method        string
	Path          string
	Resource      string
	PartnerID     string
	Status        int
	Outcome       string
	OldState      any
	NewState      any
	DurationMS    int64
	CreatedAt     time.Time
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status == 0 { w.status = status }
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 { w.status = http.StatusOK }
	if w.body.Len() < 65536 {
		remaining := 65536 - w.body.Len()
		if len(p) < remaining { remaining = len(p) }
		if remaining > 0 { _, _ = w.body.Write(p[:remaining]) }
	}
	return w.ResponseWriter.Write(p)
}

func (w *auditResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type user struct {
	ID, Name, Email, PasswordHash string
	Roles                         []string
	Active                        bool
	SystemOwner                   bool
	PreferredLocale               string
	Timezone                      string
	JobTitle                      string
	Phone                         string
	SessionVersion                int
}
type claims struct {
	Sub, Email, Name string
	Roles            []string
	Version          int `json:"v"`
	Exp              int64
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	ttlHours, _ := strconv.Atoi(common.Env("HIMATE_SESSION_TTL_HOURS", "8"))
	rememberTTLHours, _ := strconv.Atoi(common.Env("HIMATE_REMEMBER_TTL_HOURS", "720"))
	if rememberTTLHours < ttlHours { rememberTTLHours = ttlHours }
	secure, _ := strconv.ParseBool(common.Env("COOKIE_SECURE", "true"))
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	}
	a := &app{
		db: db, secret: os.Getenv("HIMATE_SESSION_SECRET"), internalToken: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		webDir: common.Env("WEB_DIST_DIR", "/app/web"), env: common.Env("HIMATE_ENV", "development"),
		version: common.Env("HIMATE_APP_VERSION", "0.2.0-start-04-08"),
		ttl: time.Duration(ttlHours) * time.Hour, rememberTTL: time.Duration(rememberTTLHours) * time.Hour,
		secureCookie: secure, client: &http.Client{Timeout: 4 * time.Second, Transport: transport},
		proxies: map[string]*httputil.ReverseProxy{},
		loginAttempts: map[string]loginState{},
		auditQueue: make(chan auditEvent, 4096),
		hosts: map[string]string{
			"partners":     os.Getenv("PARTNERS_HOSTPORT"),
			"catalog":      os.Getenv("CATALOG_HOSTPORT"),
			"billing":      os.Getenv("BILLING_HOSTPORT"),
			"contact":      os.Getenv("CONTACT_HOSTPORT"),
			"provisioning": os.Getenv("PROVISIONING_HOSTPORT"),
			"environments": os.Getenv("ENVIRONMENTS_HOSTPORT"),
			"connector":    os.Getenv("CONNECTOR_HOSTPORT"),
			"health":       os.Getenv("HEALTH_HOSTPORT"),
			"impact":       os.Getenv("IMPACT_HOSTPORT"),
			"evidence":     os.Getenv("EVIDENCE_HOSTPORT"),
			"reports":      os.Getenv("REPORTS_HOSTPORT"),
			"cms":          os.Getenv("CMS_HOSTPORT"),
			"storage":      os.Getenv("STORAGE_HOSTPORT"),
			"partner-runtime": os.Getenv("PARTNER_RUNTIME_HOSTPORT"),
		},
	}
	if len(a.secret) < 32 || len(a.internalToken) < 24 {
		log.Error("required secrets are missing")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	go a.auditWriter()
	for name, host := range a.hosts {
		if strings.TrimSpace(host) == "" {
			log.Warn("private service host is not configured", "service", name)
			continue
		}
		p, err := newProxy(host, a.internalToken)
		if err != nil {
			log.Warn("private service proxy is unavailable", "service", name, "error", err)
			continue
		}
		a.proxies[name] = p
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/live", a.live)
	mux.HandleFunc("/api/v1/health", a.health)
	mux.HandleFunc("/api/v1/auth/login", a.login)
	mux.HandleFunc("/api/v1/auth/logout", a.logout)
	mux.HandleFunc("/api/v1/auth/me", a.me)
	mux.HandleFunc("/api/v1/public/contact", a.publicContact)
	mux.HandleFunc("/robots.txt", a.robots)
	mux.HandleFunc("/sitemap.xml", a.sitemap)
	mux.HandleFunc("/public/v1/cms/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "cms")
	})
	mux.HandleFunc("/preview/v1/cms/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "cms")
	})
	mux.HandleFunc("/connector/v1/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "connector")
	})
	mux.HandleFunc("/api/", a.api)
	mux.Handle("/", a.web())
	common.Run(log, "gateway", common.Env("PORT", "10000"), securityHeaders(mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ApplyMigrations(ctx, a.db, "identity", []common.Migration{
		{Version: 1, Name: "identity-base", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS identity`,
			`CREATE TABLE IF NOT EXISTS identity.users(
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				roles JSONB NOT NULL DEFAULT '["platform_admin"]'::jsonb,
				active BOOLEAN NOT NULL DEFAULT TRUE
			)`,
			`CREATE INDEX IF NOT EXISTS identity_users_active_idx ON identity.users(active)`,
		}},
		{Version: 2, Name: "central-audit-log", Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.audit_events(
				id BIGSERIAL PRIMARY KEY,
				actor_id TEXT NOT NULL DEFAULT '',
				actor_name TEXT NOT NULL DEFAULT '',
				actor_roles JSONB NOT NULL DEFAULT '[]'::jsonb,
				request_id TEXT NOT NULL DEFAULT '',
				method TEXT NOT NULL,
				path TEXT NOT NULL,
				resource TEXT NOT NULL DEFAULT '',
				partner_id TEXT NOT NULL DEFAULT '',
				status INTEGER NOT NULL,
				outcome TEXT NOT NULL,
				duration_ms BIGINT NOT NULL DEFAULT 0,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_created_idx ON identity.audit_events(created_at DESC,id DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_actor_idx ON identity.audit_events(actor_id,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_resource_idx ON identity.audit_events(resource,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_partner_idx ON identity.audit_events(partner_id,created_at DESC) WHERE partner_id<>''`,
			`CREATE INDEX IF NOT EXISTS identity_audit_request_idx ON identity.audit_events(request_id) WHERE request_id<>''`,
		}},
		{Version: 3, Name: "rbac-user-lifecycle", Statements: []string{
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
			`CREATE INDEX IF NOT EXISTS identity_users_roles_idx ON identity.users USING gin(roles)`,
			`CREATE INDEX IF NOT EXISTS identity_users_name_idx ON identity.users(lower(name),lower(email))`,
		}},
		{Version: 4, Name: "audit-integrity-and-state", Statements: []string{
			`ALTER TABLE identity.audit_events ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.audit_events ADD COLUMN IF NOT EXISTS correlation_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.audit_events ADD COLUMN IF NOT EXISTS old_state JSONB NOT NULL DEFAULT '{}'::jsonb`,
			`ALTER TABLE identity.audit_events ADD COLUMN IF NOT EXISTS new_state JSONB NOT NULL DEFAULT '{}'::jsonb`,
			`UPDATE identity.audit_events SET action=UPPER(resource||'_'||method) WHERE action=''`,
			`UPDATE identity.audit_events SET correlation_id=request_id WHERE correlation_id='' AND request_id<>''`,
			`CREATE INDEX IF NOT EXISTS identity_audit_action_idx ON identity.audit_events(action,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_correlation_idx ON identity.audit_events(correlation_id) WHERE correlation_id<>''`,
			`CREATE OR REPLACE FUNCTION identity.reject_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'identity.audit_events is append-only'; RETURN OLD; END; $$`,
			`DROP TRIGGER IF EXISTS identity_audit_append_only ON identity.audit_events`,
			`CREATE TRIGGER identity_audit_append_only BEFORE UPDATE OR DELETE ON identity.audit_events FOR EACH ROW EXECUTE FUNCTION identity.reject_audit_mutation()`,
		}},
		{Version: 5, Name: "user-profile-and-owner", Statements: []string{
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS system_owner BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS preferred_locale TEXT NOT NULL DEFAULT 'en_US'`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS timezone TEXT NOT NULL DEFAULT 'UTC'`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS job_title TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS session_version INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ`,
			`CREATE UNIQUE INDEX IF NOT EXISTS identity_single_system_owner_idx ON identity.users(system_owner) WHERE system_owner=TRUE`,
			`CREATE INDEX IF NOT EXISTS identity_users_locale_idx ON identity.users(preferred_locale)`,
		}},
	}); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(os.Getenv("HIMATE_BOOTSTRAP_ADMIN_EMAIL")))
	password := os.Getenv("HIMATE_BOOTSTRAP_ADMIN_PASSWORD")
	name := common.Env("HIMATE_BOOTSTRAP_ADMIN_NAME", "HIMATE Administrator")
	if !strings.Contains(email, "@") || len(password) < 12 {
		return errors.New("HIMATE_BOOTSTRAP_ADMIN_EMAIL and a 12+ character HIMATE_BOOTSTRAP_ADMIN_PASSWORD are required")
	}
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	roles, _ := json.Marshal([]string{"platform_admin"})
	_, err = a.db.ExecContext(ctx, `INSERT INTO identity.users(id,name,email,password_hash,roles,active,preferred_locale,timezone)
		VALUES('usr_bootstrap_001',$1,$2,$3,$4::jsonb,TRUE,'en_US','UTC')
		ON CONFLICT(email) DO NOTHING`,
		name, email, hashed, string(roles))
	if err != nil { return err }
	var ownerCount int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity.users WHERE system_owner=TRUE`).Scan(&ownerCount); err != nil {
		return err
	}
	if ownerCount == 0 {
		_, err = a.db.ExecContext(ctx, `UPDATE identity.users SET system_owner=TRUE,roles='["platform_admin"]'::jsonb,active=TRUE,updated_at=NOW()
			WHERE lower(email)=lower($1)`, email)
	}
	return err
}

func clientKey(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		// Use the proxy-appended hop rather than the client-controlled leftmost
		// value so a forged X-Forwarded-For cannot trivially bypass throttling.
		if candidate := strings.TrimSpace(parts[len(parts)-1]); candidate != "" {
			return candidate
		}
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 { host = host[:i] }
	return strings.Trim(host, "[]")
}

func (a *app) loginAllowed(key string, now time.Time) bool {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	state := a.loginAttempts[key]
	if now.Before(state.BlockedUntil) { return false }
	if state.WindowStart.IsZero() || now.Sub(state.WindowStart) > 10*time.Minute {
		delete(a.loginAttempts, key)
		return true
	}
	return true
}

func (a *app) recordLoginFailure(key string, now time.Time) {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	state := a.loginAttempts[key]
	if state.WindowStart.IsZero() || now.Sub(state.WindowStart) > 10*time.Minute {
		state = loginState{WindowStart: now}
	}
	state.Failures++
	if state.Failures >= 5 {
		state.BlockedUntil = now.Add(15 * time.Minute)
		state.Failures = 0
		state.WindowStart = now
	}
	a.loginAttempts[key] = state
}

func (a *app) clearLoginFailures(key string) {
	a.loginMu.Lock()
	delete(a.loginAttempts, key)
	a.loginMu.Unlock()
}

func requestOriginAllowed(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" { return true }
	u, err := url.Parse(origin)
	if err != nil { return false }
	return strings.EqualFold(u.Host, r.Host)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	if !requestOriginAllowed(r) {
		common.APIError(w, 403, "CSRF", "Cross-site request rejected")
		return
	}
	key, now := clientKey(r), time.Now().UTC()
	if !a.loginAllowed(key, now) {
		w.Header().Set("Retry-After", "900")
		common.APIError(w, 429, "RATE_LIMITED", "Too many sign-in attempts. Try again later.")
		return
	}
	var in struct {
		Email string `json:"email"`
		Password string `json:"password"`
		Remember bool `json:"remember"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, 400, "JSON", "Invalid request")
		return
	}
	u, err := a.findUser("email", strings.ToLower(strings.TrimSpace(in.Email)))
	valid := err == nil && u.Active && verifyPassword(u.PasswordHash, in.Password)
	if err != nil {
		// Keep missing-account and wrong-password work factors closer together.
		_ = pbkdf2SHA256([]byte(in.Password), make([]byte, 16), passwordIterations, 32)
	}
	if !valid {
		a.recordLoginFailure(key, now)
		common.APIError(w, 401, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}
	a.clearLoginFailures(key)
	sessionTTL := a.ttl
	if in.Remember { sessionTTL = a.rememberTTL }
	token, _ := a.issueSession(u, sessionTTL)
	cookie := &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: a.secureCookie, SameSite: http.SameSiteStrictMode,
	}
	if in.Remember {
		cookie.MaxAge = int(sessionTTL.Seconds())
		cookie.Expires = time.Now().UTC().Add(sessionTTL)
	}
	http.SetCookie(w, cookie)
	common.JSON(w, 200, publicUser(u))
}
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	if !requestOriginAllowed(r) {
		common.APIError(w, 403, "CSRF", "Cross-site request rejected")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (a *app) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.auth(r)
	if err != nil {
		common.APIError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	common.JSON(w, 200, publicUser(u))
}

type roleDefinition struct {
	Key         string
	Label       string
	Description string
	Permissions []string
}

var roleDefinitions = []roleDefinition{
	{
		Key: "platform_admin", Label: "Platform Admin",
		Description: "Full HIMATE control-plane administration, governance and user access.",
		Permissions: []string{"*"},
	},
	{
		Key: "operations_admin", Label: "Operations Admin",
		Description: "Partner operations, modules, provisioning, environments, connectors and system health.",
		Permissions: []string{
			"dashboard.read",
			"partners.read", "partners.write", "partners.approve",
			"catalog.read", "catalog.write", "catalog.approve",
			"provisioning.read", "provisioning.write", "provisioning.approve",
			"environments.read", "environments.write", "environments.approve",
			"connectors.read", "connectors.write", "connectors.approve",
			"health.read",
		},
	},
	{
		Key: "finance_admin", Label: "Finance Admin",
		Description: "Partner commercial terms, licenses, subscriptions, billing and finance data.",
		Permissions: []string{
			"dashboard.read", "partners.read", "catalog.read",
			"billing.read", "billing.write", "billing.approve",
		},
	},
	{
		Key: "reporting_admin", Label: "Reporting Admin",
		Description: "Impact metrics, evidence verification and report generation.",
		Permissions: []string{
			"dashboard.read", "partners.read",
			"impact.read", "impact.write", "impact.approve",
			"evidence.read", "evidence.write", "evidence.approve",
			"reports.read", "reports.write", "reports.approve",
		},
	},
}

func roleDefinitionByKey(key string) (roleDefinition, bool) {
	for _, definition := range roleDefinitions {
		if definition.Key == key { return definition, true }
	}
	return roleDefinition{}, false
}

func normalizeRoles(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" || seen[key] { continue }
		if _, ok := roleDefinitionByKey(key); !ok {
			return nil, fmt.Errorf("unknown role %q", key)
		}
		seen[key] = true
		out = append(out, key)
	}
	if len(out) == 0 { return nil, errors.New("at least one role is required") }
	return out, nil
}

func permissionsForRoles(roles []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, role := range roles {
		definition, ok := roleDefinitionByKey(role)
		if !ok { continue }
		for _, permission := range definition.Permissions {
			if permission == "*" {
				return []string{"*"}
			}
			if !seen[permission] {
				seen[permission] = true
				out = append(out, permission)
			}
		}
	}
	return out
}

func containsRole(roles []string, role string) bool {
	for _, value := range roles { if value==role { return true } }
	return false
}

func hasRole(u user, role string) bool {
	for _, value := range u.Roles {
		if value == role { return true }
	}
	return false
}

func hasPermission(u user, required string) bool {
	if strings.TrimSpace(required) == "" { return true }
	for _, role := range u.Roles {
		definition, ok := roleDefinitionByKey(role)
		if !ok { continue }
		for _, permission := range definition.Permissions {
			if permission == "*" || permission == required { return true }
		}
	}
	return false
}

func permissionResource(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/api/v1/dashboard/summary":
		return "dashboard"
	case path == "/api/v1/audit/events":
		return "audit"
	case path == "/api/v1/admin/roles", path == "/api/v1/admin/users", strings.HasPrefix(path, "/api/v1/admin/users/"):
		return "administration"
	case strings.HasPrefix(path, "/api/v1/partners/") && strings.Contains(path, "/modules"):
		return "catalog"
	case path == "/api/v1/modules", path == "/api/v1/module-groups", strings.HasPrefix(path, "/api/v1/modules/"):
		return "catalog"
	case path == "/api/v1/partner-categories", path == "/api/v1/partners", strings.HasPrefix(path, "/api/v1/partners/"):
		return "partners"
	case strings.HasPrefix(path, "/api/v1/billing/"):
		return "billing"
	case strings.HasPrefix(path, "/api/v1/provisioning/"):
		return "provisioning"
	case path == "/api/v1/environments", strings.HasPrefix(path, "/api/v1/environments/"):
		return "environments"
	case strings.HasPrefix(path, "/api/v1/connectors/"):
		return "connectors"
	case strings.HasPrefix(path, "/api/v1/system-health"):
		return "health"
	case strings.HasPrefix(path, "/api/v1/impact/"):
		return "impact"
	case path == "/api/v1/evidence", strings.HasPrefix(path, "/api/v1/evidence/"):
		return "evidence"
	case path == "/api/v1/reports", strings.HasPrefix(path, "/api/v1/reports/"):
		return "reports"
	case path == "/api/v1/cms/pages", strings.HasPrefix(path, "/api/v1/cms/pages/"),
		path == "/api/v1/cms/media", strings.HasPrefix(path, "/api/v1/cms/media/"):
		return "cms"
	default:
		return ""
	}
}

func requiredPermission(r *http.Request) string {
	resource := permissionResource(r)
	if resource == "" { return "" }
	action := "read"
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		action = "read"
	case http.MethodDelete:
		action = "approve"
	default:
		action = "write"
	}

	path := r.URL.Path
	switch {
	case resource == "administration" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions:
		action = "approve"
	case resource == "cms" && (strings.HasSuffix(path, "/publish") || strings.HasSuffix(path, "/rollback")):
		action = "approve"
	case resource == "provisioning" && strings.HasSuffix(path, "/run"):
		action = "approve"
	case resource == "environments" && (strings.HasSuffix(path, "/deploy") || strings.HasSuffix(path, "/launch")):
		action = "approve"
	case resource == "evidence" && r.Method == http.MethodPatch:
		action = "approve"
	case resource == "billing" && strings.Contains(path, "/license") && r.Method == http.MethodPut:
		action = "approve"
	}
	return resource + "." + action
}


func auditSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, part := range []string{"password","token","secret","authorization","cookie","credential","api_key","apikey"} {
		if strings.Contains(key, part) { return true }
	}
	return false
}

func sanitizeAuditValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if auditSensitiveKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = sanitizeAuditValue(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed { out[i] = sanitizeAuditValue(item) }
		return out
	default:
		return value
	}
}

func captureAuditRequest(r *http.Request) any {
	if r == nil || r.Body == nil || !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return map[string]any{}
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return map[string]any{"capture_error": "request body unavailable"}
	}
	if len(raw) > 65536 {
		return map[string]any{"truncated": true}
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return map[string]any{}
	}
	return sanitizeAuditValue(decoded)
}

func decodeAuditState(raw []byte) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return map[string]any{}
	}
	return sanitizeAuditValue(decoded)
}

func auditAction(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/api/v1/profile" && r.Method == http.MethodPatch:
		return "PROFILE_UPDATED"
	case path == "/api/v1/profile/password" && r.Method == http.MethodPost:
		return "PROFILE_PASSWORD_CHANGED"
	case path == "/api/v1/admin/users" && r.Method == http.MethodPost:
		return "ADMIN_USER_CREATED"
	case strings.HasPrefix(path, "/api/v1/admin/users/") && r.Method == http.MethodPatch:
		return "ADMIN_USER_UPDATED"
	case strings.HasSuffix(path, "/publish"):
		return "CMS_PAGE_PUBLISHED"
	case strings.HasSuffix(path, "/rollback"):
		return "CMS_PAGE_ROLLBACK_PUBLISHED"
	case strings.HasSuffix(path, "/run") && strings.Contains(path, "/provisioning/"):
		return "PROVISIONING_RUN"
	case strings.HasSuffix(path, "/verify-domain"):
		return "DOMAIN_VERIFIED"
	case strings.HasSuffix(path, "/deploy") && strings.Contains(path, "/environments/"):
		return "ENVIRONMENT_DEPLOY"
	case strings.HasSuffix(path, "/launch") && strings.Contains(path, "/environments/"):
		return "ENVIRONMENT_LAUNCH"
	case strings.HasPrefix(path, "/api/v1/evidence/") && r.Method == http.MethodPatch:
		return "EVIDENCE_VERIFICATION_CHANGED"
	case strings.Contains(path, "/license") && r.Method == http.MethodPut:
		return "LICENSE_CHANGED"
	}
	resource, _ := auditResource(r)
	resource = strings.ToUpper(strings.ReplaceAll(resource, "-", "_"))
	if resource == "" { resource = "API" }
	return resource + "_" + strings.ToUpper(r.Method)
}

func auditUserState(u user) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":append([]string(nil),u.Roles...),"active":u.Active,
		"system_owner":u.SystemOwner,"preferred_locale":normalizedLocale(u.PreferredLocale),"timezone":normalizedTimezone(u.Timezone),
		"job_title":u.JobTitle,"phone":u.Phone,"permissions":permissionsForRoles(u.Roles),
	}
}

func (a *app) auditOldState(r *http.Request) any {
	if r == nil { return map[string]any{} }
	if r.Method == http.MethodPatch && r.URL.Path == "/api/v1/profile" {
		if u, err := a.auth(r); err == nil { return auditUserState(u) }
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/profile/password" {
		if u, err := a.auth(r); err == nil { return auditUserState(u) }
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/") {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/")
		if id != "" && !strings.Contains(id, "/") {
			if u, err := a.findUser("id", id); err == nil { return auditUserState(u) }
		}
	}
	return map[string]any{}
}

func (a *app) api(w http.ResponseWriter, r *http.Request) {
	u, err := a.auth(r)
	if err != nil {
		common.APIError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	if !requestOriginAllowed(r) {
		common.APIError(w, 403, "CSRF", "Cross-site request rejected")
		return
	}

	mutating := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
	if mutating {
		started := time.Now()
		requestState := captureAuditRequest(r)
		oldState := a.auditOldState(r)
		recorder := &auditResponseWriter{ResponseWriter: w}
		w = recorder
		defer func() {
			status := recorder.status
			if status == 0 { status = http.StatusOK }
			resource, partnerID := auditResource(r)
			outcome := "SUCCESS"
			if status >= 400 { outcome = "FAILED" }
			newState := decodeAuditState(recorder.body.Bytes())
			if state, ok := newState.(map[string]any); ok && len(state) == 0 {
				newState = requestState
			}
			a.enqueueAudit(auditEvent{
				ActorID: u.ID, ActorName: u.Name, ActorRoles: append([]string(nil), u.Roles...),
				RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
				CorrelationID: strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
				Action: auditAction(r),
				Method: r.Method, Path: r.URL.Path, Resource: resource, PartnerID: partnerID,
				Status: status, Outcome: outcome, OldState: oldState, NewState: newState,
				DurationMS: time.Since(started).Milliseconds(), CreatedAt: time.Now().UTC(),
			})
		}()
	}
	required := requiredPermission(r)
	if required != "" && !hasPermission(u, required) {
		common.APIError(w, 403, "FORBIDDEN", "Required permission: "+required)
		return
	}
	r.Header.Set("X-Himate-User-ID", u.ID)
	if r.URL.Path == "/api/v1/dashboard/summary" {
		a.dashboard(w, r)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/profile":
		a.profile(w,r,u)
	case r.URL.Path == "/api/v1/profile/password":
		a.profilePassword(w,r,u)
	case r.URL.Path == "/api/v1/admin/roles" && r.Method == http.MethodGet:
		a.adminRoles(w, r)
	case r.URL.Path == "/api/v1/admin/users":
		a.adminUsers(w, r, u)
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/"):
		a.adminUser(w, r, u)
	case r.URL.Path == "/api/v1/audit/events" && r.Method == http.MethodGet:
		a.auditEvents(w, r)
	case r.URL.Path == "/api/v1/partners/portfolio" && r.Method == http.MethodGet:
		a.partnerPortfolioMetrics(w, r)
	case r.URL.Path == "/api/v1/partners" && r.Method == http.MethodGet:
		a.partnerPortfolio(w, r)
	case r.URL.Path == "/api/v1/partners", r.URL.Path == "/api/v1/partner-categories":
		a.serveProxy(w, r, "partners")
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/") && strings.Contains(r.URL.Path, "/modules"):
		a.serveProxy(w, r, "catalog")
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/"):
		a.serveProxy(w, r, "partners")
	case r.URL.Path == "/api/v1/modules", r.URL.Path == "/api/v1/module-groups", strings.HasPrefix(r.URL.Path, "/api/v1/modules/"):
		a.serveProxy(w, r, "catalog")
	case strings.HasPrefix(r.URL.Path, "/api/v1/billing/"):
		a.serveProxy(w, r, "billing")
	case strings.HasPrefix(r.URL.Path, "/api/v1/provisioning/"):
		a.serveProxy(w, r, "provisioning")
	case r.URL.Path == "/api/v1/environments", strings.HasPrefix(r.URL.Path, "/api/v1/environments/"):
		a.serveProxy(w, r, "environments")
	case strings.HasPrefix(r.URL.Path, "/api/v1/connectors/"):
		a.serveProxy(w, r, "connector")
	case strings.HasPrefix(r.URL.Path, "/api/v1/system-health"):
		a.serveProxy(w, r, "health")
	case strings.HasPrefix(r.URL.Path, "/api/v1/impact/"):
		a.serveProxy(w, r, "impact")
	case r.URL.Path == "/api/v1/evidence", strings.HasPrefix(r.URL.Path, "/api/v1/evidence/"):
		a.serveProxy(w, r, "evidence")
	case r.URL.Path == "/api/v1/reports", strings.HasPrefix(r.URL.Path, "/api/v1/reports/"):
		a.serveProxy(w, r, "reports")
	case r.URL.Path == "/api/v1/cms/pages", strings.HasPrefix(r.URL.Path, "/api/v1/cms/pages/"),
		r.URL.Path == "/api/v1/cms/media", strings.HasPrefix(r.URL.Path, "/api/v1/cms/media/"):
		a.serveProxy(w, r, "cms")
	default:
		common.APIError(w, 404, "API_NOT_FOUND", "API endpoint not found")
	}
}

func auditResource(r *http.Request) (string, string) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" { return "api", "" }
	resource := parts[0]
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	switch parts[0] {
	case "partners":
		resource = "partners"
		if len(parts) > 1 && parts[1] != "portfolio" { partnerID = parts[1] }
	case "billing":
		resource = "billing"
		if len(parts) > 2 && parts[1] == "partners" { partnerID = parts[2] }
	case "connectors":
		resource = "connectors"
		if len(parts) > 1 { partnerID = parts[1] }
	case "cms":
		resource = "cms"
	case "impact":
		resource = "impact"
	case "evidence":
		resource = "evidence"
	case "reports":
		resource = "reports"
	case "provisioning":
		resource = "provisioning"
	case "environments":
		resource = "environments"
	case "modules", "module-groups":
		resource = "catalog"
	case "partner-categories":
		resource = "partners"
	}
	return resource, partnerID
}

func (a *app) enqueueAudit(event auditEvent) {
	if a == nil || a.db == nil || a.auditQueue == nil { return }
	select {
	case a.auditQueue <- event:
	default:
		go a.persistAudit(event)
	}
}

func (a *app) auditWriter() {
	for event := range a.auditQueue {
		a.persistAudit(event)
	}
}

func (a *app) persistAudit(event auditEvent) {
	if a == nil || a.db == nil { return }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	roles, _ := json.Marshal(event.ActorRoles)
	oldState, _ := json.Marshal(sanitizeAuditValue(event.OldState))
	newState, _ := json.Marshal(sanitizeAuditValue(event.NewState))
	_, _ = a.db.ExecContext(ctx, `INSERT INTO identity.audit_events(
		actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,status,outcome,old_state,new_state,duration_ms,created_at
	) VALUES($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb,$15,$16)`,
		event.ActorID,event.ActorName,string(roles),event.RequestID,event.CorrelationID,event.Action,event.Method,event.Path,event.Resource,event.PartnerID,
		event.Status,event.Outcome,string(oldState),string(newState),event.DurationMS,event.CreatedAt)
}

func auditLimit(value string, fallback, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 { return fallback }
	if n > max { return max }
	return n
}

func (a *app) auditEvents(w http.ResponseWriter, r *http.Request) {
	limit := auditLimit(r.URL.Query().Get("limit"), 50, 200)
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	if offset < 0 { offset = 0 }

	where := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(actor_name ILIKE $%d OR actor_id ILIKE $%d OR path ILIKE $%d OR resource ILIKE $%d OR request_id ILIKE $%d OR correlation_id ILIKE $%d OR action ILIKE $%d OR partner_id ILIKE $%d)", n,n,n,n,n,n,n,n))
	}
	if actorID := strings.TrimSpace(r.URL.Query().Get("actor_id")); actorID != "" { add("actor_id=$%d", actorID) }
	if resource := strings.TrimSpace(r.URL.Query().Get("resource")); resource != "" { add("resource=$%d", resource) }
	if action := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("action"))); action != "" { add("action=$%d", action) }
	if correlationID := strings.TrimSpace(r.URL.Query().Get("correlation_id")); correlationID != "" { add("correlation_id=$%d", correlationID) }
	if method := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("method"))); method != "" { add("method=$%d", method) }
	if outcome := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("outcome"))); outcome != "" { add("outcome=$%d", outcome) }
	if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" { add("partner_id=$%d", partnerID) }
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil { common.APIError(w,400,"VALIDATION","from must be RFC3339"); return }
		add("created_at >= $%d", value.UTC())
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil { common.APIError(w,400,"VALIDATION","to must be RFC3339"); return }
		add("created_at <= $%d", value.UTC())
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM identity.audit_events WHERE "+whereSQL, args...).Scan(&total); err != nil {
		common.APIError(w,500,"DB","Could not load audit count")
		return
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := a.db.Query(`SELECT id,actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,status,outcome,old_state,new_state,duration_ms,created_at
		FROM identity.audit_events WHERE `+whereSQL+` ORDER BY created_at DESC,id DESC LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		common.APIError(w,500,"DB","Could not load audit events")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var actorID,actorName,requestID,correlationID,action,method,path,resource,partnerID,outcome string
		var rolesRaw,oldRaw,newRaw []byte
		var status int
		var duration int64
		var created time.Time
		if rows.Scan(&id,&actorID,&actorName,&rolesRaw,&requestID,&correlationID,&action,&method,&path,&resource,&partnerID,&status,&outcome,&oldRaw,&newRaw,&duration,&created) != nil { continue }
		var roles []string
		var oldState,newState any
		_ = json.Unmarshal(rolesRaw,&roles)
		_ = json.Unmarshal(oldRaw,&oldState)
		_ = json.Unmarshal(newRaw,&newState)
		items = append(items,map[string]any{
			"id":id,"actor_id":actorID,"actor_name":actorName,"actor_roles":roles,
			"request_id":requestID,"correlation_id":correlationID,"action":action,
			"method":method,"path":path,"resource":resource,"partner_id":partnerID,"status":status,
			"outcome":outcome,"old_state":oldState,"new_state":newState,
			"duration_ms":duration,"created_at":created.UTC(),
		})
	}
	common.JSON(w,200,map[string]any{
		"items":items,"count":len(items),"total":total,"limit":limit,"offset":offset,"has_more":offset+len(items)<total,
	})
}

func (a *app) live(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"service": "himate-gateway",
		"environment": a.env,
		"version": a.version,
		"time": time.Now().UTC(),
	})
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{"identity": "ok"}
	overall := "ok"
	checkedAt := time.Now().UTC()
	for name, host := range a.hosts {
		if strings.TrimSpace(host) == "" {
			services[name] = "unconfigured"
			overall = "degraded"
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/health", nil)
		if err != nil {
			cancel()
			services[name] = "unavailable"
			overall = "degraded"
			continue
		}
		resp, err := a.client.Do(req)
		cancel()
		if err != nil || resp.StatusCode >= 300 {
			services[name] = "unavailable"
			overall = "degraded"
		} else {
			services[name] = "ok"
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	common.JSON(w, 200, map[string]any{"status": overall, "service": "himate-gateway", "environment": a.env, "version": a.version, "architecture": "containerized-microservices-start-20", "checked_at": checkedAt, "services": services})
}

func (a *app) partnerPortfolio(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	type partnerPage struct {
		Items           []map[string]any `json:"items"`
		Count           int              `json:"count"`
		Total           int              `json:"total"`
		Limit           int              `json:"limit"`
		Offset          int              `json:"offset"`
		HasMore         bool             `json:"has_more"`
		LifecycleCounts map[string]int   `json:"lifecycle_counts"`
		ReferenceCount  int              `json:"reference_count"`
	}
	type portfolioPage struct { Items []map[string]any `json:"items"` }

	var partners partnerPage
	query := r.URL.Query()
	coreOnly := strings.EqualFold(strings.TrimSpace(query.Get("core_only")), "true")
	query.Del("core_only")
	path := "/api/v1/partners"
	if encoded := query.Encode(); encoded != "" { path += "?" + encoded }
	if err := a.internalGET(ctx, a.hosts["partners"], path, &partners); err != nil {
		common.APIError(w, 502, "PARTNERS_UNAVAILABLE", "Partner portfolio is temporarily unavailable")
		return
	}
	if coreOnly {
		w.Header().Set("Server-Timing", fmt.Sprintf("partner-core;dur=%d", time.Since(started).Milliseconds()))
		common.JSON(w, 200, map[string]any{
			"items": partners.Items, "count": partners.Count, "total": partners.Total,
			"limit": partners.Limit, "offset": partners.Offset, "has_more": partners.HasMore,
			"lifecycle_counts": partners.LifecycleCounts, "reference_count": partners.ReferenceCount,
		})
		return
	}

	var catalogPortfolio, billingPortfolio, healthPortfolio portfolioPage
	var catalogErr, billingErr, healthErr error
	if len(partners.Items) > 0 {
		ids := make([]string, 0, len(partners.Items))
		for _, item := range partners.Items {
			if id := strings.TrimSpace(fmt.Sprint(item["id"])); id != "" { ids = append(ids, id) }
		}
		filter := url.QueryEscape(strings.Join(ids, ","))
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			catalogErr = a.internalGET(ctx, a.hosts["catalog"], "/internal/v1/portfolio?ids="+filter, &catalogPortfolio)
		}()
		go func() {
			defer wg.Done()
			billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/portfolio?ids="+filter, &billingPortfolio)
		}()
		go func() {
			defer wg.Done()
			healthErr = a.internalGET(ctx, a.hosts["health"], "/internal/v1/system-health/partner-snapshots?ids="+filter, &healthPortfolio)
		}()
		wg.Wait()
	}

	catalogByID := map[string]map[string]any{}
	for _, item := range catalogPortfolio.Items { catalogByID[fmt.Sprint(item["partner_id"])] = item }
	billingByID := map[string]map[string]any{}
	for _, item := range billingPortfolio.Items { billingByID[fmt.Sprint(item["partner_id"])] = item }
	healthByID := map[string]map[string]any{}
	for _, item := range healthPortfolio.Items { healthByID[fmt.Sprint(item["partner_id"])] = item }

	for _, p := range partners.Items {
		id := fmt.Sprint(p["id"])
		cat := catalogByID[id]
		bill := billingByID[id]
		hlt := healthByID[id]
		active := 0
		extra, base := 0.0, 0.0
		if v, ok := cat["active_modules"].(float64); ok { active = int(v) }
		if v, ok := cat["extra_module_fee"].(float64); ok { extra = v }
		if v, ok := bill["effective_base_fee"].(float64); ok { base = v }
		p["active_modules"] = active
		p["base_service_fee"] = base
		p["extra_module_fee"] = extra
		p["service_value_30d"] = mathRound2(base + extra)
		if currency := fmt.Sprint(bill["currency"]); currency != "<nil>" { p["currency"] = currency }
		if hlt != nil {
			if v := strings.TrimSpace(fmt.Sprint(hlt["overall_status"])); v != "" && v != "<nil>" { p["system_health"] = v }
			if v := strings.TrimSpace(fmt.Sprint(hlt["platform_version"])); v != "" && v != "<nil>" { p["platform_version"] = v }
			p["connector_health"] = hlt["connector_health"]
			p["environment_status"] = hlt["environment_status"]
			p["provisioning_status"] = hlt["provisioning_status"]
		}
	}
	if catalogErr != nil || billingErr != nil || healthErr != nil {
		w.Header().Set("X-Himate-Portfolio", "partial")
	}
	w.Header().Set("Server-Timing", fmt.Sprintf("partner-portfolio;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, 200, map[string]any{
		"items": partners.Items, "count": partners.Count, "total": partners.Total,
		"limit": partners.Limit, "offset": partners.Offset, "has_more": partners.HasMore,
		"lifecycle_counts": partners.LifecycleCounts, "reference_count": partners.ReferenceCount,
	})
}

func (a *app) partnerPortfolioMetrics(w http.ResponseWriter, r *http.Request) {
	rawIDs := strings.TrimSpace(r.URL.Query().Get("ids"))
	if rawIDs == "" {
		common.JSON(w, 200, map[string]any{"items": []map[string]any{}, "count": 0})
		return
	}
	ids := make([]string, 0, 32)
	seen := map[string]bool{}
	for _, raw := range strings.Split(rawIDs, ",") {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] { continue }
		seen[id] = true
		ids = append(ids, id)
		if len(ids) >= 200 { break }
	}
	type portfolioPage struct { Items []map[string]any `json:"items"` }
	ctx, cancel := context.WithTimeout(r.Context(), 2500*time.Millisecond)
	defer cancel()
	filter := url.QueryEscape(strings.Join(ids, ","))
	var catalogPortfolio, billingPortfolio, healthPortfolio portfolioPage
	var wg sync.WaitGroup
	wg.Add(3)
	go func(){ defer wg.Done(); _ = a.internalGET(ctx,a.hosts["catalog"],"/internal/v1/portfolio?ids="+filter,&catalogPortfolio) }()
	go func(){ defer wg.Done(); _ = a.internalGET(ctx,a.hosts["billing"],"/internal/v1/portfolio?ids="+filter,&billingPortfolio) }()
	go func(){ defer wg.Done(); _ = a.internalGET(ctx,a.hosts["health"],"/internal/v1/system-health/partner-snapshots?ids="+filter,&healthPortfolio) }()
	wg.Wait()
	catalogByID:=map[string]map[string]any{}; for _,x:=range catalogPortfolio.Items{catalogByID[fmt.Sprint(x["partner_id"])]=x}
	billingByID:=map[string]map[string]any{}; for _,x:=range billingPortfolio.Items{billingByID[fmt.Sprint(x["partner_id"])]=x}
	healthByID:=map[string]map[string]any{}; for _,x:=range healthPortfolio.Items{healthByID[fmt.Sprint(x["partner_id"])]=x}
	items:=make([]map[string]any,0,len(ids))
	for _,id:=range ids{
		out:=map[string]any{"partner_id":id}
		cat,bill,hlt:=catalogByID[id],billingByID[id],healthByID[id]
		active:=0; extra,base:=0.0,0.0
		if v,ok:=cat["active_modules"].(float64);ok{active=int(v)}
		if v,ok:=cat["extra_module_fee"].(float64);ok{extra=v}
		if v,ok:=bill["effective_base_fee"].(float64);ok{base=v}
		out["active_modules"]=active; out["base_service_fee"]=base; out["extra_module_fee"]=extra; out["service_value_30d"]=mathRound2(base+extra)
		if v:=strings.TrimSpace(fmt.Sprint(hlt["overall_status"]));v!=""&&v!="<nil>"{out["system_health"]=v}
		if v:=strings.TrimSpace(fmt.Sprint(hlt["platform_version"]));v!=""&&v!="<nil>"{out["platform_version"]=v}
		items=append(items,out)
	}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func mathRound2(v float64) float64 {
	if v >= 0 { return float64(int64(v*100+0.5)) / 100 }
	return float64(int64(v*100-0.5)) / 100
}

func (a *app) dashboard(w http.ResponseWriter, r *http.Request) {
	a.dashboardMu.RLock()
	if a.dashboardPayload != nil && time.Now().Before(a.dashboardExpires) {
		payload := a.dashboardPayload
		a.dashboardMu.RUnlock()
		w.Header().Set("X-Himate-Cache", "hit")
		common.JSON(w, http.StatusOK, payload)
		return
	}
	stale := a.dashboardPayload
	a.dashboardMu.RUnlock()

	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var partnerResponse struct {
		Items           []map[string]any `json:"items"`
		Total           int              `json:"total"`
		LifecycleCounts map[string]int   `json:"lifecycle_counts"`
	}
	var moduleResponse struct {
		Items []map[string]any `json:"items"`
		Count int              `json:"count"`
	}
	var partnerErr, moduleErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		partnerErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partners?limit=1&offset=0&include_archived=true", &partnerResponse)
	}()
	go func() {
		defer wg.Done()
		moduleErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &moduleResponse)
	}()
	wg.Wait()

	if partnerErr != nil && moduleErr != nil && stale != nil {
		w.Header().Set("X-Himate-Cache", "stale")
		w.Header().Set("Server-Timing", fmt.Sprintf("dashboard;dur=%d", time.Since(started).Milliseconds()))
		common.JSON(w, http.StatusOK, stale)
		return
	}

	live := partnerResponse.LifecycleCounts["LIVE"]
	status := "healthy"
	if partnerErr != nil || moduleErr != nil {
		status = "degraded"
	}
	payload := map[string]any{
		"partners": map[string]any{"total": partnerResponse.Total, "live": live, "lifecycle_counts": partnerResponse.LifecycleCounts},
		"modules": map[string]any{"catalog_total": moduleResponse.Count},
		"system": map[string]any{
			"status": status, "environment": a.env, "version": a.version, "architecture": "containerized-microservices-start-09-13",
		},
	}

	a.dashboardMu.Lock()
	a.dashboardPayload = payload
	a.dashboardExpires = time.Now().Add(10 * time.Second)
	a.dashboardMu.Unlock()

	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("dashboard;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) publicContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	proxy := a.proxies["contact"]
	if proxy == nil {
		common.APIError(w, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "Contact service is unavailable")
		return
	}
	proxy.ServeHTTP(w, r)
}
func (a *app) internalGET(ctx context.Context, host, path string, dst any) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("private service host is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Himate-Internal-Token", a.internalToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (a *app) serveProxy(w http.ResponseWriter, r *http.Request, service string) {
	proxy := a.proxies[service]
	if proxy == nil {
		common.APIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", service+" service is temporarily unavailable")
		return
	}
	proxy.ServeHTTP(w, r)
}

func newProxy(host, token string) (*httputil.ReverseProxy, error) {
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("private service host is required")
	}
	target, err := url.Parse("http://" + host)
	if err != nil {
		return nil, err
	}
	p := httputil.NewSingleHostReverseProxy(target)
	base := p.Director
	p.Director = func(r *http.Request) { base(r); r.Header.Set("X-Himate-Internal-Token", token) }
	return p, nil
}


func normalizedLocale(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hu", "hu-hu", "hu_hu":
		return "hu_HU"
	default:
		return "en_US"
	}
}

func normalizedTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" { return "UTC" }
	if len(value) > 64 || strings.IndexFunc(value, unicode.IsSpace) >= 0 { return "UTC" }
	for _, r := range value {
		if !((r>='A'&&r<='Z')||(r>='a'&&r<='z')||(r>='0'&&r<='9')||r=='/'||r=='_'||r=='-'||r=='+') {
			return "UTC"
		}
	}
	return value
}

func ownerRequired(w http.ResponseWriter, actor user) bool {
	if actor.SystemOwner { return true }
	common.APIError(w,http.StatusForbidden,"OWNER_REQUIRED","Only the HIMATE system owner can manage administration users")
	return false
}

func (a *app) profile(w http.ResponseWriter, r *http.Request, actor user) {
	switch r.Method {
	case http.MethodGet:
		common.JSON(w,http.StatusOK,publicUser(actor))
	case http.MethodPatch:
		var in struct {
			Name *string `json:"name"`
			PreferredLocale *string `json:"preferred_locale"`
			Timezone *string `json:"timezone"`
			JobTitle *string `json:"job_title"`
			Phone *string `json:"phone"`
		}
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		next:=actor
		if in.Name!=nil {
			next.Name=strings.TrimSpace(*in.Name)
			if len(next.Name)<2||len(next.Name)>120 { common.APIError(w,400,"VALIDATION","Name must be 2-120 characters");return }
		}
		if in.PreferredLocale!=nil {
			raw:=strings.TrimSpace(*in.PreferredLocale)
			if raw!="en_US"&&raw!="hu_HU" { common.APIError(w,400,"VALIDATION","preferred_locale must be en_US or hu_HU");return }
			next.PreferredLocale=raw
		}
		if in.Timezone!=nil {
			raw:=strings.TrimSpace(*in.Timezone)
			next.Timezone=normalizedTimezone(raw)
			if raw!=""&&next.Timezone=="UTC"&&raw!="UTC" { common.APIError(w,400,"VALIDATION","Invalid timezone");return }
		}
		if in.JobTitle!=nil {
			next.JobTitle=strings.TrimSpace(*in.JobTitle)
			if len(next.JobTitle)>120 { common.APIError(w,400,"VALIDATION","Job title is too long");return }
		}
		if in.Phone!=nil {
			next.Phone=strings.TrimSpace(*in.Phone)
			if len(next.Phone)>50 { common.APIError(w,400,"VALIDATION","Phone is too long");return }
		}
		_,err:=a.db.Exec(`UPDATE identity.users SET name=$2,preferred_locale=$3,timezone=$4,job_title=$5,phone=$6,updated_at=NOW() WHERE id=$1`,
			actor.ID,next.Name,next.PreferredLocale,next.Timezone,next.JobTitle,next.Phone)
		if err!=nil { common.APIError(w,500,"DB","Could not update profile");return }
		common.JSON(w,http.StatusOK,publicUser(next))
	default:
		common.APIError(w,405,"METHOD","Use GET or PATCH")
	}
}

func (a *app) profilePassword(w http.ResponseWriter, r *http.Request, actor user) {
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword string `json:"new_password"`
	}
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
	if !verifyPassword(actor.PasswordHash,in.CurrentPassword) {
		common.APIError(w,403,"CURRENT_PASSWORD","Current password is incorrect");return
	}
	if len(in.NewPassword)<12 {
		common.APIError(w,400,"VALIDATION","New password must be at least 12 characters");return
	}
	if subtle.ConstantTimeCompare([]byte(in.CurrentPassword),[]byte(in.NewPassword))==1 {
		common.APIError(w,400,"VALIDATION","New password must be different");return
	}
	hash,err:=hashPassword(in.NewPassword)
	if err!=nil { common.APIError(w,500,"PASSWORD","Could not secure password");return }
	_,err=a.db.Exec(`UPDATE identity.users SET password_hash=$2,session_version=session_version+1,password_changed_at=NOW(),updated_at=NOW() WHERE id=$1`,actor.ID,hash)
	if err!=nil { common.APIError(w,500,"DB","Could not change password");return }
	next,err:=a.findUser("id",actor.ID)
	if err!=nil { common.APIError(w,500,"DB","Could not refresh profile");return }
	token,err:=a.issueSession(next,a.ttl)
	if err!=nil { common.APIError(w,500,"SESSION","Could not refresh session");return }
	http.SetCookie(w,&http.Cookie{Name:sessionCookie,Value:token,Path:"/",HttpOnly:true,Secure:a.secureCookie,SameSite:http.SameSiteStrictMode})
	common.JSON(w,http.StatusOK,publicUser(next))
}

func adminUserMap(u user, createdAt, updatedAt time.Time) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":u.Roles,"active":u.Active,
		"system_owner":u.SystemOwner,"preferred_locale":normalizedLocale(u.PreferredLocale),
		"timezone":normalizedTimezone(u.Timezone),"job_title":u.JobTitle,"phone":u.Phone,
		"permissions":permissionsForRoles(u.Roles),
		"created_at":createdAt.UTC(),"updated_at":updatedAt.UTC(),
	}
}

func newUserID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil { return "", err }
	return "usr_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func validEmail(value string) bool {
	value = strings.TrimSpace(value)
	at := strings.LastIndex(value, "@")
	return at > 0 && at < len(value)-3 && strings.Contains(value[at+1:], ".")
}

func (a *app) adminRoles(w http.ResponseWriter, r *http.Request) {
	items := make([]map[string]any, 0, len(roleDefinitions))
	for _, role := range roleDefinitions {
		items = append(items, map[string]any{
			"key": role.Key,
			"label": role.Label,
			"description": role.Description,
			"permissions": role.Permissions,
		})
	}
	common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *app) adminUsers(w http.ResponseWriter, r *http.Request, actor user) {
	if !ownerRequired(w,actor) { return }
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,email,password_hash,roles,active,system_owner,preferred_locale,timezone,job_title,phone,session_version,created_at,updated_at
			FROM identity.users ORDER BY system_owner DESC,active DESC,lower(name),lower(email)`)
		if err != nil { common.APIError(w,500,"DB","Could not load administration users"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var u user
			var rolesRaw []byte
			var createdAt, updatedAt time.Time
			if rows.Scan(&u.ID,&u.Name,&u.Email,&u.PasswordHash,&rolesRaw,&u.Active,&u.SystemOwner,&u.PreferredLocale,&u.Timezone,&u.JobTitle,&u.Phone,&u.SessionVersion,&createdAt,&updatedAt) != nil { continue }
			_ = json.Unmarshal(rolesRaw,&u.Roles)
			items = append(items, adminUserMap(u,createdAt,updatedAt))
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct {
			Name string `json:"name"`
			Email string `json:"email"`
			Password string `json:"password"`
			Roles []string `json:"roles"`
		}
		if common.Decode(r,&in) != nil { common.APIError(w,400,"JSON","Invalid request"); return }
		in.Name = strings.TrimSpace(in.Name)
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if len(in.Name) < 2 || len(in.Name) > 120 { common.APIError(w,400,"VALIDATION","Name must be 2-120 characters"); return }
		if !validEmail(in.Email) { common.APIError(w,400,"VALIDATION","A valid email is required"); return }
		if len(in.Password) < 12 { common.APIError(w,400,"VALIDATION","Password must be at least 12 characters"); return }
		roles, err := normalizeRoles(in.Roles)
		if err != nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
		if containsRole(roles,"platform_admin") { common.APIError(w,409,"OWNER_ROLE_RESERVED","Platform Admin is reserved for the HIMATE system owner"); return }
		var exists bool
		_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1))`,in.Email).Scan(&exists)
		if exists { common.APIError(w,409,"EMAIL_EXISTS","An administrator with this email already exists"); return }
		hash, err := hashPassword(in.Password)
		if err != nil { common.APIError(w,500,"PASSWORD","Could not secure password"); return }
		id, err := newUserID()
		if err != nil { common.APIError(w,500,"ID","Could not create user ID"); return }
		rolesRaw, _ := json.Marshal(roles)
		var createdAt, updatedAt time.Time
		err = a.db.QueryRow(`INSERT INTO identity.users(id,name,email,password_hash,roles,active)
			VALUES($1,$2,$3,$4,$5::jsonb,TRUE)
			RETURNING created_at,updated_at`,id,in.Name,in.Email,hash,string(rolesRaw)).Scan(&createdAt,&updatedAt)
		if err != nil { common.APIError(w,500,"DB","Could not create administration user"); return }
		u := user{ID:id,Name:in.Name,Email:in.Email,PasswordHash:hash,Roles:roles,Active:true,PreferredLocale:"en_US",Timezone:"UTC"}
		common.JSON(w,201,adminUserMap(u,createdAt,updatedAt))
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) adminUser(w http.ResponseWriter, r *http.Request, actor user) {
	if !ownerRequired(w,actor) { return }
	if r.Method != http.MethodPatch { common.APIError(w,405,"METHOD","Use PATCH"); return }
	id := strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/admin/users/"),"/")
	if id == "" || strings.Contains(id,"/") { common.APIError(w,404,"NOT_FOUND","Administration user not found"); return }

	current, err := a.findUser("id",id)
	if err != nil { common.APIError(w,404,"NOT_FOUND","Administration user not found"); return }

	var in struct {
		Name *string `json:"name"`
		Email *string `json:"email"`
		Password *string `json:"password"`
		Roles *[]string `json:"roles"`
		Active *bool `json:"active"`
	}
	if common.Decode(r,&in) != nil { common.APIError(w,400,"JSON","Invalid request"); return }

	next := current
	if in.Name != nil {
		next.Name = strings.TrimSpace(*in.Name)
		if len(next.Name) < 2 || len(next.Name) > 120 { common.APIError(w,400,"VALIDATION","Name must be 2-120 characters"); return }
	}
	if in.Email != nil {
		next.Email = strings.ToLower(strings.TrimSpace(*in.Email))
		if !validEmail(next.Email) { common.APIError(w,400,"VALIDATION","A valid email is required"); return }
		var duplicate bool
		_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1) AND id<>$2)`,next.Email,id).Scan(&duplicate)
		if duplicate { common.APIError(w,409,"EMAIL_EXISTS","An administrator with this email already exists"); return }
	}
	if in.Roles != nil {
		next.Roles, err = normalizeRoles(*in.Roles)
		if err != nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
		if !current.SystemOwner && containsRole(next.Roles,"platform_admin") { common.APIError(w,409,"OWNER_ROLE_RESERVED","Platform Admin is reserved for the HIMATE system owner"); return }
	}
	if in.Active != nil { next.Active = *in.Active }
	if current.SystemOwner && (!next.Active || !containsRole(next.Roles,"platform_admin")) {
		common.APIError(w,409,"OWNER_PROTECTED","The HIMATE system owner must remain an active Platform Admin")
		return
	}

	currentPlatform := current.Active && hasRole(current,"platform_admin")
	nextPlatform := next.Active && hasRole(next,"platform_admin")
	if currentPlatform && !nextPlatform {
		var otherPlatformAdmins int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM identity.users
			WHERE id<>$1 AND active=TRUE AND roles @> '["platform_admin"]'::jsonb`,id).Scan(&otherPlatformAdmins); err != nil {
			common.APIError(w,500,"DB","Could not validate Platform Admin continuity")
			return
		}
		if otherPlatformAdmins == 0 {
			common.APIError(w,409,"LAST_PLATFORM_ADMIN","At least one active Platform Admin must remain")
			return
		}
	}

	hash := current.PasswordHash
	if in.Password != nil {
		if len(*in.Password) < 12 { common.APIError(w,400,"VALIDATION","Password must be at least 12 characters"); return }
		hash, err = hashPassword(*in.Password)
		if err != nil { common.APIError(w,500,"PASSWORD","Could not secure password"); return }
	}
	rolesRaw, _ := json.Marshal(next.Roles)
	var createdAt, updatedAt time.Time
	err = a.db.QueryRow(`UPDATE identity.users SET name=$2,email=$3,password_hash=$4,roles=$5::jsonb,active=$6,updated_at=NOW()
		WHERE id=$1 RETURNING created_at,updated_at`,id,next.Name,next.Email,hash,string(rolesRaw),next.Active).Scan(&createdAt,&updatedAt)
	if err != nil { common.APIError(w,500,"DB","Could not update administration user"); return }
	next.PasswordHash = hash
	common.JSON(w,200,adminUserMap(next,createdAt,updatedAt))
}

func (a *app) findUser(field, value string) (user, error) {
	if field != "email" && field != "id" {
		return user{}, errors.New("invalid lookup")
	}
	var u user
	var raw []byte
	err := a.db.QueryRow(`SELECT id,name,email,password_hash,roles,active,system_owner,preferred_locale,timezone,job_title,phone,session_version
		FROM identity.users WHERE `+field+`=$1`, value).
		Scan(&u.ID,&u.Name,&u.Email,&u.PasswordHash,&raw,&u.Active,&u.SystemOwner,&u.PreferredLocale,&u.Timezone,&u.JobTitle,&u.Phone,&u.SessionVersion)
	_ = json.Unmarshal(raw, &u.Roles)
	return u, err
}
func (a *app) auth(r *http.Request) (user, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return user{}, err
	}
	c, err := a.parseSession(cookie.Value)
	if err != nil {
		return user{}, err
	}
	u, err := a.findUser("id", c.Sub)
	if err != nil || !u.Active {
		return user{}, errors.New("inactive")
	}
	if c.Version != u.SessionVersion {
		return user{}, errors.New("session superseded")
	}
	return u, nil
}
func publicUser(u user) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":u.Roles,
		"permissions":permissionsForRoles(u.Roles),
		"system_owner":u.SystemOwner,
		"preferred_locale":normalizedLocale(u.PreferredLocale),
		"timezone":normalizedTimezone(u.Timezone),
		"job_title":u.JobTitle,
		"phone":u.Phone,
		"can_manage_users":u.SystemOwner,
	}
}

func (a *app) issueSession(u user, ttl time.Duration) (string, error) {
	if ttl <= 0 { ttl = a.ttl }
	raw, _ := json.Marshal(claims{Sub:u.ID,Email:u.Email,Name:u.Name,Roles:u.Roles,Version:u.SessionVersion,Exp:time.Now().Add(ttl).Unix()})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(a.secret))
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (a *app) parseSession(token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return claims{}, errors.New("invalid session")
	}
	mac := hmac.New(sha256.New, []byte(a.secret))
	mac.Write([]byte(parts[0]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), sig) {
		return claims{}, errors.New("invalid signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims{}, err
	}
	var c claims
	if err = json.Unmarshal(raw, &c); err != nil || time.Now().Unix() >= c.Exp {
		return claims{}, errors.New("expired session")
	}
	return c, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2SHA256([]byte(password), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func pbkdf2SHA256(password, salt []byte, iterations, length int) []byte {
	hashLen := sha256.Size
	blocks := (length + hashLen - 1) / hashLen
	out := make([]byte, 0, blocks*hashLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:length]
}


type publicCMSSEO struct {
	Title           string `json:"title"`
	MetaDescription string `json:"meta_description"`
	Canonical       string `json:"canonical"`
	OGTitle         string `json:"og_title"`
	OGDescription   string `json:"og_description"`
	OGImageAssetID  string `json:"og_image_asset_id"`
	NoIndex         bool   `json:"noindex"`
}

type publicCMSSection struct {
	ID            string `json:"id"`
	ComponentType string `json:"component_type"`
	Heading       string `json:"heading"`
	Body          string `json:"body"`
	MediaAssetID  string `json:"media_asset_id"`
	CTALabel      string `json:"cta_label"`
	CTAURL        string `json:"cta_url"`
	Visible       bool   `json:"visible"`
	SortOrder     int    `json:"sort_order"`
}

type publicCMSPage struct {
	Slug           string             `json:"slug"`
	SEO            publicCMSSEO       `json:"seo"`
	Sections       []publicCMSSection `json:"sections"`
	HiddenSections []string           `json:"hidden_sections"`
}

type publicCMSManifest struct {
	Items []struct {
		Slug      string `json:"slug"`
		Canonical string `json:"canonical"`
		Title     string `json:"title"`
		NoIndex   bool   `json:"noindex"`
	} `json:"items"`
}

func (a *app) fetchPublishedCMS(ctx context.Context, slug string) (publicCMSPage, error) {
	var out publicCMSPage
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/public/v1/cms/pages/"+url.PathEscape(slug), nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.internalToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("CMS page %s returned %d", slug, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (a *app) fetchPublishedManifest(ctx context.Context) (publicCMSManifest, error) {
	var out publicCMSManifest
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/public/v1/cms/manifest", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.internalToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("CMS manifest returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func publicOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func replaceHeadTag(doc, marker, replacement string) string {
	lower := strings.ToLower(doc)
	idx := strings.Index(lower, strings.ToLower(marker))
	if idx >= 0 {
		start := strings.LastIndex(doc[:idx], "<")
		endRel := strings.Index(doc[idx:], ">")
		if start >= 0 && endRel >= 0 {
			end := idx + endRel + 1
			return doc[:start] + replacement + doc[end:]
		}
	}
	if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
		return doc[:headEnd] + replacement + doc[headEnd:]
	}
	return doc
}

func replaceTitle(doc, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return doc
	}
	lower := strings.ToLower(doc)
	start := strings.Index(lower, "<title>")
	end := strings.Index(lower, "</title>")
	tag := "<title>" + html.EscapeString(value) + "</title>"
	if start >= 0 && end > start {
		return doc[:start] + tag + doc[end+len("</title>"):]
	}
	if headEnd := strings.Index(lower, "</head>"); headEnd >= 0 {
		return doc[:headEnd] + tag + doc[headEnd:]
	}
	return doc
}

func replaceFirstTagText(fragment, tag, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fragment
	}
	lower := strings.ToLower(fragment)
	open := strings.Index(lower, "<"+tag)
	if open < 0 {
		return fragment
	}
	openEndRel := strings.Index(fragment[open:], ">")
	if openEndRel < 0 {
		return fragment
	}
	contentStart := open + openEndRel + 1
	closeTag := "</" + tag + ">"
	closeRel := strings.Index(strings.ToLower(fragment[contentStart:]), closeTag)
	if closeRel < 0 {
		return fragment
	}
	contentEnd := contentStart + closeRel
	escaped := strings.ReplaceAll(html.EscapeString(value), "\n", "<br>")
	return fragment[:contentStart] + escaped + fragment[contentEnd:]
}

func replaceFirstAttribute(fragment, tag, attr, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fragment
	}
	lower := strings.ToLower(fragment)
	open := strings.Index(lower, "<"+tag)
	if open < 0 {
		return fragment
	}
	openEndRel := strings.Index(fragment[open:], ">")
	if openEndRel < 0 {
		return fragment
	}
	openEnd := open + openEndRel + 1
	opening := fragment[open:openEnd]
	attrNeedle := attr + "=\""
	attrPos := strings.Index(strings.ToLower(opening), strings.ToLower(attrNeedle))
	escaped := html.EscapeString(value)
	if attrPos >= 0 {
		valueStart := attrPos + len(attrNeedle)
		valueEndRel := strings.Index(opening[valueStart:], "\"")
		if valueEndRel >= 0 {
			valueEnd := valueStart + valueEndRel
			opening = opening[:valueStart] + escaped + opening[valueEnd:]
		}
	} else {
		opening = strings.TrimSuffix(opening, ">") + " " + attr + "=\"" + escaped + "\">"
	}
	return fragment[:open] + opening + fragment[openEnd:]
}

func marketingSectionBounds(doc, id string) (int, int, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, 0, false
	}
	needle := "data-cms-section=\"" + id + "\""
	idx := strings.Index(doc, needle)
	if idx < 0 {
		return 0, 0, false
	}
	start := strings.LastIndex(doc[:idx], "<section")
	if start < 0 {
		return 0, 0, false
	}
	endRel := strings.Index(strings.ToLower(doc[idx:]), "</section>")
	if endRel < 0 {
		return 0, 0, false
	}
	end := idx + endRel + len("</section>")
	return start, end, true
}

func removeMarketingSection(doc, id string) string {
	start, end, ok := marketingSectionBounds(doc, id)
	if !ok {
		return doc
	}
	return doc[:start] + doc[end:]
}

func renderMarketingSection(doc string, section publicCMSSection) string {
	start, end, ok := marketingSectionBounds(doc, section.ID)
	if !ok {
		return doc
	}
	fragment := doc[start:end]
	for _, tag := range []string{"h1", "h2", "h3"} {
		next := replaceFirstTagText(fragment, tag, section.Heading)
		if next != fragment {
			fragment = next
			break
		}
	}
	fragment = replaceFirstTagText(fragment, "p", section.Body)
	if section.CTALabel != "" {
		fragment = replaceFirstTagText(fragment, "a", section.CTALabel)
	}
	if section.CTAURL != "" {
		fragment = replaceFirstAttribute(fragment, "a", "href", section.CTAURL)
	}
	if section.MediaAssetID != "" {
		fragment = replaceFirstAttribute(fragment, "img", "src", "/public/v1/cms/media/"+url.PathEscape(section.MediaAssetID))
	}
	return doc[:start] + fragment + doc[end:]
}

func renderPublishedCMSHTML(doc string, page publicCMSPage, requestURL string) string {
	doc = replaceTitle(doc, page.SEO.Title)
	if value := strings.TrimSpace(page.SEO.MetaDescription); value != "" {
		doc = replaceHeadTag(doc, "name=\"description\"", "<meta name=\"description\" content=\""+html.EscapeString(value)+"\">")
	}
	canonical := strings.TrimSpace(page.SEO.Canonical)
	if canonical == "" {
		canonical = requestURL
	}
	doc = replaceHeadTag(doc, "rel=\"canonical\"", "<link rel=\"canonical\" href=\""+html.EscapeString(canonical)+"\">")
	robots := "index,follow"
	if page.SEO.NoIndex {
		robots = "noindex,nofollow"
	}
	doc = replaceHeadTag(doc, "name=\"robots\"", "<meta name=\"robots\" content=\""+robots+"\">")
	ogTitle := strings.TrimSpace(page.SEO.OGTitle)
	if ogTitle == "" {
		ogTitle = strings.TrimSpace(page.SEO.Title)
	}
	if ogTitle != "" {
		doc = replaceHeadTag(doc, "property=\"og:title\"", "<meta property=\"og:title\" content=\""+html.EscapeString(ogTitle)+"\">")
	}
	ogDescription := strings.TrimSpace(page.SEO.OGDescription)
	if ogDescription == "" {
		ogDescription = strings.TrimSpace(page.SEO.MetaDescription)
	}
	if ogDescription != "" {
		doc = replaceHeadTag(doc, "property=\"og:description\"", "<meta property=\"og:description\" content=\""+html.EscapeString(ogDescription)+"\">")
	}
	doc = replaceHeadTag(doc, "property=\"og:url\"", "<meta property=\"og:url\" content=\""+html.EscapeString(canonical)+"\">")
	if mediaID := strings.TrimSpace(page.SEO.OGImageAssetID); mediaID != "" {
		doc = replaceHeadTag(doc, "property=\"og:image\"", "<meta property=\"og:image\" content=\"/public/v1/cms/media/"+url.PathEscape(mediaID)+"\">")
	}
	for _, id := range page.HiddenSections {
		doc = removeMarketingSection(doc, id)
	}
	for _, section := range page.Sections {
		doc = renderMarketingSection(doc, section)
	}
	if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
		doc = doc[:headEnd] + "<!-- HIMATE SSR:PUBLISHED -->" + doc[headEnd:]
	}
	return doc
}

func (a *app) serveMarketingPage(w http.ResponseWriter, r *http.Request, filename, slug string) {
	path := filepath.Join(filepath.Clean(a.webDir), filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	doc := string(raw)
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	page, cmsErr := a.fetchPublishedCMS(ctx, slug)
	cancel()
	if cmsErr == nil {
		doc = renderPublishedCMSHTML(doc, page, publicOrigin(r)+r.URL.Path)
		w.Header().Set("X-Himate-SSR", "published")
	} else {
		w.Header().Set("X-Himate-SSR", "static-fallback")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len([]byte(doc))))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(doc))
}

func (a *app) robots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	body := "User-agent: *\nAllow: /\nSitemap: " + publicOrigin(r) + "/sitemap.xml\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

func (a *app) sitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	manifest, err := a.fetchPublishedManifest(ctx)
	cancel()
	if err != nil {
		common.APIError(w, http.StatusServiceUnavailable, "CMS_UNAVAILABLE", "Published CMS manifest is unavailable")
		return
	}
	origin := publicOrigin(r)
	var body strings.Builder
	body.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	body.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, item := range manifest.Items {
		if item.NoIndex {
			continue
		}
		loc := strings.TrimSpace(item.Canonical)
		if loc == "" {
			if strings.EqualFold(strings.TrimSpace(item.Slug), "landing") {
				loc = origin + "/"
			} else {
				loc = origin + "/" + strings.Trim(strings.TrimSpace(item.Slug), "/")
			}
		}
		body.WriteString("<url><loc>")
		body.WriteString(html.EscapeString(loc))
		body.WriteString("</loc></url>")
	}
	body.WriteString("</urlset>")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body.String()))
	}
}


func (a *app) web() http.Handler {
	root := filepath.Clean(a.webDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			a.serveMarketingPage(w, r, "landing.html", "landing")
			return
		}

		if r.URL.Path == "/login" || r.URL.Path == "/app" || strings.HasPrefix(r.URL.Path, "/app/") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}

		if r.URL.Path == "/technology" || r.URL.Path == "/security" {
			http.Redirect(w, r, "/platform", http.StatusPermanentRedirect)
			return
		}

		marketingPages := map[string]string{
			"/platform": "platform.html",
			"/modules":  "modules.html",
			"/programs": "programs.html",
			"/impact":   "impact.html",
			"/partners": "partners.html",
			"/contact":  "contact.html",
		}
		if page, ok := marketingPages[r.URL.Path]; ok {
			a.serveMarketingPage(w, r, page, strings.TrimPrefix(r.URL.Path, "/"))
			return
		}

		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." {
			clean = "index.html"
		}
		file := filepath.Join(root, clean)
		if strings.HasPrefix(file, root) {
			if info, err := os.Stat(file); err == nil && !info.IsDir() {
				if ct := mime.TypeByExtension(filepath.Ext(file)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				http.ServeFile(w, r, file)
				return
			}
		}

		if strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasPrefix(r.URL.Path, "/canvaskit/") ||
			strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".wasm") ||
			strings.HasSuffix(r.URL.Path, ".json") ||
			strings.HasSuffix(r.URL.Path, ".ico") ||
			strings.HasSuffix(r.URL.Path, ".png") ||
			strings.HasSuffix(r.URL.Path, ".jpg") ||
			strings.HasSuffix(r.URL.Path, ".jpeg") ||
			strings.HasSuffix(r.URL.Path, ".webp") ||
			strings.HasSuffix(r.URL.Path, ".svg") {
			http.NotFound(w, r)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			raw := make([]byte, 12)
			if _, err := rand.Read(raw); err == nil { requestID = base64.RawURLEncoding.EncodeToString(raw) }
		}
		if requestID != "" {
			r.Header.Set("X-Request-ID", requestID)
			w.Header().Set("X-Request-ID", requestID)
		}
		correlationID := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
		if correlationID == "" && requestID != "" {
			correlationID = "corr_" + requestID
		}
		if correlationID != "" {
			r.Header.Set("X-Correlation-ID", correlationID)
			w.Header().Set("X-Correlation-ID", correlationID)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		} else if strings.HasPrefix(path, "/brand/") {
			w.Header().Set("Cache-Control", "no-store, max-age=0, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else if strings.HasPrefix(path, "/art/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "/" || path == "/login" || path == "/app" || strings.HasPrefix(path, "/app/") || path == "/platform" || path == "/modules" || path == "/programs" || path == "/impact" || path == "/partners" || path == "/contact" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "no-store, max-age=0, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".wasm") {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; form-action 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; connect-src 'self' https://fonts.gstatic.com; font-src 'self' data: https://fonts.gstatic.com; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
