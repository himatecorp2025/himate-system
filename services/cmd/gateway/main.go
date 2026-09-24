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
	"net"
	"net/http"
	"net/http/httputil"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	passwordResetTTL time.Duration
	resetBaseURL     string
	smtpHost         string
	smtpPort         string
	smtpUser         string
	smtpPass         string
	smtpFrom         string
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
	resetTTLMinutes, _ := strconv.Atoi(common.Env("HIMATE_PASSWORD_RESET_TTL_MINUTES", "30"))
	if resetTTLMinutes < 10 { resetTTLMinutes = 10 }
	if resetTTLMinutes > 120 { resetTTLMinutes = 120 }
	secure, _ := strconv.ParseBool(common.Env("COOKIE_SECURE", "true"))
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	}
	a := &app{
		db: db, secret: os.Getenv("HIMATE_SESSION_SECRET"), internalToken: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		webDir: common.Env("WEB_DIST_DIR", "/app/web"), env: common.Env("HIMATE_ENV", "development"),
		version: common.Env("HIMATE_APP_VERSION", "0.8.32-start-23.11.7"),
		ttl: time.Duration(ttlHours) * time.Hour, rememberTTL: time.Duration(rememberTTLHours) * time.Hour,
		passwordResetTTL: time.Duration(resetTTLMinutes) * time.Minute,
		resetBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("HIMATE_PASSWORD_RESET_BASE_URL")), "/"),
		smtpHost: strings.TrimSpace(os.Getenv("SMTP_HOST")), smtpPort: common.Env("SMTP_PORT", "587"),
		smtpUser: strings.TrimSpace(os.Getenv("SMTP_USERNAME")), smtpPass: os.Getenv("SMTP_PASSWORD"),
		smtpFrom: strings.TrimSpace(os.Getenv("SMTP_FROM")),
		secureCookie: secure, client: &http.Client{Timeout: 4 * time.Second, Transport: transport},
		proxies: map[string]*httputil.ReverseProxy{},
		loginAttempts: map[string]loginState{},
		auditQueue: make(chan auditEvent, 4096),
		hosts: map[string]string{
			"partners":     os.Getenv("PARTNERS_HOSTPORT"),
			"catalog":      os.Getenv("CATALOG_HOSTPORT"),
			"billing":      os.Getenv("BILLING_HOSTPORT"),
			"payments":     os.Getenv("PAYMENTS_HOSTPORT"),
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
			"backups":      os.Getenv("BACKUPS_HOSTPORT"),
			"partner-runtime": os.Getenv("PARTNER_RUNTIME_HOSTPORT"),
			"notifications":   os.Getenv("NOTIFICATIONS_HOSTPORT"),
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
		p, err := newProxy(host, a.internalToken, a.version)
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
	mux.HandleFunc("/api/v1/auth/password-reset/request", a.passwordResetRequest)
	mux.HandleFunc("/api/v1/auth/password-reset/confirm", a.passwordResetConfirm)
	mux.HandleFunc("/partner/api/v1/auth/login", a.partnerLogin)
	mux.HandleFunc("/partner/api/v1/auth/logout", a.partnerLogout)
	mux.HandleFunc("/partner/api/v1/auth/me", a.partnerMe)
	mux.HandleFunc("/partner/api/v1/", a.partnerAPI)
	mux.HandleFunc("/api/v1/public/contact", a.publicContact)
	mux.HandleFunc("/robots.txt", a.robots)
	mux.HandleFunc("/sitemap.xml", a.sitemap)
	mux.HandleFunc("/public/v1/cms/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "cms")
	})
	mux.HandleFunc("/preview/v1/cms/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "cms")
	})
	mux.HandleFunc("/cms-preview/", a.cmsPagePreview)
	mux.HandleFunc("/design-preview", a.designPreview)
	mux.HandleFunc("/connector/v1/", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "connector")
	})
	mux.HandleFunc("/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
		a.serveProxy(w, r, "payments")
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
		{Version: 6, Name: "custom-rbac-roles", Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.custom_roles(
				role_key TEXT PRIMARY KEY,
				label TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
				active BOOLEAN NOT NULL DEFAULT TRUE,
				created_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_custom_roles_active_idx ON identity.custom_roles(active,role_key)`,
		}},
		partnerPortalMigration(),
		{Version: 8, Name: "start-23-5-bilingual-custom-roles", Statements: []string{
			`ALTER TABLE identity.custom_roles ADD COLUMN IF NOT EXISTS label_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.custom_roles ADD COLUMN IF NOT EXISTS label_hu TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.custom_roles ADD COLUMN IF NOT EXISTS description_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.custom_roles ADD COLUMN IF NOT EXISTS description_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE identity.custom_roles SET label_en=label WHERE label_en=''`,
			`UPDATE identity.custom_roles SET label_hu=label WHERE label_hu=''`,
			`UPDATE identity.custom_roles SET description_en=description WHERE description_en=''`,
			`UPDATE identity.custom_roles SET description_hu=description WHERE description_hu=''`,
		}},
		{Version: 9, Name: "start-23-6-password-reset-tokens", Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.password_reset_tokens(
				id BIGSERIAL PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
				token_hash TEXT UNIQUE NOT NULL,
				expires_at TIMESTAMPTZ NOT NULL,
				used_at TIMESTAMPTZ,
				request_ip TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_password_reset_user_idx ON identity.password_reset_tokens(user_id,expires_at DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_password_reset_active_idx ON identity.password_reset_tokens(token_hash,expires_at) WHERE used_at IS NULL`,
		}},
		platformSecretsMigration(),
		retiredTestPartnerIdentityMigration(),
		partnerUserModulePermissionsMigration(),
	}); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(os.Getenv("HIMATE_BOOTSTRAP_ADMIN_EMAIL")))
	password := os.Getenv("HIMATE_BOOTSTRAP_ADMIN_PASSWORD")
	name := common.Env("HIMATE_BOOTSTRAP_ADMIN_NAME", "HIMATE Administrator")
	if !validEmail(email) {
		return errors.New("a valid HIMATE_BOOTSTRAP_ADMIN_EMAIL is required")
	}
	if message := passwordPolicyError(password); message != "" {
		return fmt.Errorf("HIMATE_BOOTSTRAP_ADMIN_PASSWORD: %s", message)
	}
	var partnerEmailCollision bool
	if err := a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM identity.partner_users WHERE lower(email)=lower($1))`, email).Scan(&partnerEmailCollision); err != nil {
		return err
	}
	if partnerEmailCollision {
		return errors.New("HIMATE_BOOTSTRAP_ADMIN_EMAIL is already assigned to a Partner Portal identity")
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
	common.JSON(w, 200, a.publicUser(u))
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
	common.JSON(w, 200, a.publicUser(u))
}

func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func newPasswordResetToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { return "", err }
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *app) passwordResetDeliveryConfigured() bool {
	return a.smtpHost != "" && a.smtpPort != "" && a.smtpFrom != "" && a.resetBaseURL != ""
}

func (a *app) publishedEmailLogoURL(ctx context.Context) string {
	base := strings.TrimRight(a.resetBaseURL, "/")
	fallback := base + "/brand/himate_identity_wordmark_2026.webp"
	design, err := a.fetchPublishedDesign(ctx)
	if err != nil || design.Version <= 0 {
		return fallback
	}
	id := strings.TrimSpace(design.Design.Assets["email_logo"])
	if id == "" {
		id = strings.TrimSpace(design.Design.Assets["header_wordmark"])
	}
	if id == "" {
		id = strings.TrimSpace(design.Design.LogoMediaAssetID)
	}
	if id == "" {
		return fallback
	}
	if parsed, err := url.Parse(a.resetBaseURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		base = parsed.Scheme + "://" + parsed.Host
	}
	return strings.TrimRight(base, "/") + "/public/v1/cms/media/" + url.PathEscape(id)
}

func (a *app) sendPasswordResetEmail(to, token string) error {
	if !a.passwordResetDeliveryConfigured() {
		return errors.New("password-reset email delivery is not configured")
	}
	hostPort := net.JoinHostPort(a.smtpHost, a.smtpPort)
	var auth smtp.Auth
	if a.smtpUser != "" {
		auth = smtp.PlainAuth("", a.smtpUser, a.smtpPass, a.smtpHost)
	}
	link := a.resetBaseURL + "/login?reset_token=" + url.QueryEscape(token)
	subject := "HIMATE password reset"
	minutes := strconv.Itoa(int(a.passwordResetTTL.Minutes()))
	plain := "A password reset was requested for your HIMATE administrator account.\r\n\r\n" +
		"Open this one-time link to set a new password:\r\n" + link + "\r\n\r\n" +
		"This link expires in " + minutes + " minutes. " +
		"If you did not request this reset, you can ignore this email.\r\n"
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	logoURL := a.publishedEmailLogoURL(ctx)
	cancel()
	htmlBody := "<!doctype html><html><body style=\"margin:0;background:#f8f9fb;font-family:Arial,sans-serif;color:#1f2937\">" +
		"<div style=\"max-width:620px;margin:0 auto;padding:32px\">" +
		"<img src=\"" + html.EscapeString(logoURL) + "\" alt=\"HIMATE\" style=\"max-width:220px;height:auto;margin-bottom:28px\">" +
		"<div style=\"background:#fff;border:1px solid #e4e7ec;border-radius:10px;padding:28px\">" +
		"<h1 style=\"margin:0 0 18px;color:#0b1f3b;font-size:28px\">Password reset</h1>" +
		"<p>A password reset was requested for your HIMATE administrator account.</p>" +
		"<p><a href=\"" + html.EscapeString(link) + "\" style=\"display:inline-block;padding:12px 18px;background:#0b1f3b;color:#fff;text-decoration:none;border-radius:6px\">Set a new password</a></p>" +
		"<p>This one-time link expires in " + html.EscapeString(minutes) + " minutes.</p>" +
		"<p style=\"color:#667085\">If you did not request this reset, you can ignore this email.</p>" +
		"</div></div></body></html>"
	boundary := "himate-reset-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	msg := []byte("From: " + a.smtpFrom + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
		"--" + boundary + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + plain + "\r\n" +
		"--" + boundary + "\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + htmlBody + "\r\n" +
		"--" + boundary + "--\r\n")
	return smtp.SendMail(hostPort, auth, a.smtpFrom, []string{to}, msg)
}

func (a *app) enqueuePasswordResetAudit(r *http.Request, action, userID string, status int, outcome string) {
	a.enqueueAudit(auditEvent{
		ActorID: userID, ActorName: "Password recovery", ActorRoles: []string{"public_identity"},
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
		CorrelationID: strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
		Action: action, Method: r.Method, Path: r.URL.Path, Resource: "identity",
		Status: status, Outcome: outcome, OldState: map[string]any{}, NewState: map[string]any{},
		CreatedAt: time.Now().UTC(),
	})
}

func (a *app) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	if !requestOriginAllowed(r) {
		common.APIError(w, http.StatusForbidden, "CSRF", "Cross-site request rejected")
		return
	}
	key, now := "password-reset:"+clientKey(r), time.Now().UTC()
	if !a.loginAllowed(key, now) {
		w.Header().Set("Retry-After", "900")
		common.APIError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many password reset requests. Try again later.")
		return
	}
	a.recordLoginFailure(key, now)
	var in struct{ Email string `json:"email"` }
	if common.Decode(r, &in) != nil || !validEmail(strings.ToLower(strings.TrimSpace(in.Email))) {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "A valid email is required")
		return
	}
	if a.env == "production" && !a.passwordResetDeliveryConfigured() {
		common.APIError(w, http.StatusServiceUnavailable, "RESET_UNAVAILABLE", "Password recovery is temporarily unavailable")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	u, err := a.findUser("email", email)
	response := map[string]any{"accepted": true, "message": "If the account exists, password reset instructions have been sent."}
	if err != nil || !u.Active {
		common.JSON(w, http.StatusAccepted, response)
		return
	}
	token, err := newPasswordResetToken()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "RESET", "Could not create password reset request")
		return
	}
	tokenHash := passwordResetTokenHash(token)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not create password reset request")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE identity.password_reset_tokens SET used_at=NOW() WHERE user_id=$1 AND used_at IS NULL`, u.ID); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not create password reset request")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO identity.password_reset_tokens(user_id,token_hash,expires_at,request_ip) VALUES($1,$2,$3,$4)`,
		u.ID, tokenHash, now.Add(a.passwordResetTTL), clientKey(r)); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not create password reset request")
		return
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not create password reset request")
		return
	}
	if a.passwordResetDeliveryConfigured() {
		if err := a.sendPasswordResetEmail(u.Email, token); err != nil {
			_, _ = a.db.ExecContext(r.Context(), `UPDATE identity.password_reset_tokens SET used_at=NOW() WHERE token_hash=$1`, tokenHash)
			common.APIError(w, http.StatusServiceUnavailable, "RESET_DELIVERY", "Password recovery is temporarily unavailable")
			return
		}
	}
	if a.env != "production" {
		response["development_token"] = token
	}
	a.enqueuePasswordResetAudit(r, "PASSWORD_RESET_REQUESTED", u.ID, http.StatusAccepted, "SUCCESS")
	common.JSON(w, http.StatusAccepted, response)
}

func (a *app) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	if !requestOriginAllowed(r) {
		common.APIError(w, http.StatusForbidden, "CSRF", "Cross-site request rejected")
		return
	}
	var in struct {
		Token string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if common.Decode(r, &in) != nil || strings.TrimSpace(in.Token) == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Reset token and new password are required")
		return
	}
	if message := passwordPolicyError(in.NewPassword); message != "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", message)
		return
	}
	tokenHash := passwordResetTokenHash(strings.TrimSpace(in.Token))
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not reset password")
		return
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM identity.password_reset_tokens
		WHERE token_hash=$1 AND used_at IS NULL AND expires_at>NOW() FOR UPDATE`, tokenHash).Scan(&userID)
	if err != nil {
		a.enqueuePasswordResetAudit(r, "PASSWORD_RESET_COMPLETED", "", http.StatusBadRequest, "FAILED")
		common.APIError(w, http.StatusBadRequest, "INVALID_RESET_TOKEN", "Password reset link is invalid or expired")
		return
	}
	hash, err := hashPassword(in.NewPassword)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "PASSWORD", "Could not secure password")
		return
	}
	res, err := tx.ExecContext(r.Context(), `UPDATE identity.users SET password_hash=$2,session_version=session_version+1,password_changed_at=NOW(),updated_at=NOW()
		WHERE id=$1 AND active=TRUE`, userID, hash)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not reset password")
		return
	}
	if rows, _ := res.RowsAffected(); rows != 1 {
		common.APIError(w, http.StatusBadRequest, "INVALID_RESET_TOKEN", "Password reset link is invalid or expired")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE identity.password_reset_tokens SET used_at=NOW() WHERE user_id=$1 AND used_at IS NULL`, userID); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not finalize password reset")
		return
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not finalize password reset")
		return
	}
	a.enqueuePasswordResetAudit(r, "PASSWORD_RESET_COMPLETED", userID, http.StatusOK, "SUCCESS")
	common.JSON(w, http.StatusOK, map[string]any{"reset": true})
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
			"backups.read", "backups.write", "backups.approve",
			"health.read", "notifications.read",
		},
	},
	{
		Key: "finance_admin", Label: "Finance Admin",
		Description: "Partner commercial terms, licenses, subscriptions, billing and finance data.",
		Permissions: []string{
			"dashboard.read", "partners.read", "catalog.read",
			"billing.read", "billing.write", "billing.approve", "notifications.read",
		},
	},
	{
		Key: "reporting_admin", Label: "Reporting Admin",
		Description: "Impact metrics, evidence verification and report generation.",
		Permissions: []string{
			"dashboard.read", "partners.read",
			"impact.read", "impact.write", "impact.approve",
			"evidence.read", "evidence.write", "evidence.approve",
			"reports.read", "reports.write", "reports.approve", "notifications.read",
		},
	},
	{
		Key: "marketing_admin", Label: "Marketing Admin",
		Description: "Website content, published design settings and inbound contact-lead management.",
		Permissions: []string{
			"dashboard.read",
			"cms.read", "cms.write", "cms.approve",
			"contact.read", "contact.write", "notifications.read",
		},
	},
}

func builtinRoleDefinitionByKey(key string) (roleDefinition, bool) {
	for _, definition := range roleDefinitions {
		if definition.Key == key { return definition, true }
	}
	return roleDefinition{}, false
}

var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*\.(read|write|approve)$`)
var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func normalizeCustomPermissions(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range values {
		permission := strings.ToLower(strings.TrimSpace(raw))
		if permission == "" || seen[permission] { continue }
		if permission == "*" || !permissionPattern.MatchString(permission) {
			return nil, fmt.Errorf("invalid permission %q", permission)
		}
		if permission == "administration.approve" {
			return nil, errors.New("administration.approve is reserved for the HIMATE system owner")
		}
		seen[permission] = true
		out = append(out, permission)
	}
	if len(out) == 0 { return nil, errors.New("at least one permission is required") }
	sort.Strings(out)
	return out, nil
}

func (a *app) roleDefinitionByKey(key string) (roleDefinition, bool) {
	if definition, ok := builtinRoleDefinitionByKey(key); ok { return definition, true }
	if a == nil || a.db == nil { return roleDefinition{}, false }
	var labelEN, descriptionEN string
	var raw []byte
	var active bool
	if err := a.db.QueryRow(`SELECT label_en,description_en,permissions,active FROM identity.custom_roles WHERE role_key=$1`, key).
		Scan(&labelEN,&descriptionEN,&raw,&active); err != nil || !active {
		return roleDefinition{}, false
	}
	permissions := []string{}
	_ = json.Unmarshal(raw, &permissions)
	return roleDefinition{Key:key,Label:labelEN,Description:descriptionEN,Permissions:permissions}, true
}

func (a *app) normalizeRoles(values []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" || seen[key] { continue }
		if _, ok := a.roleDefinitionByKey(key); !ok { return nil, fmt.Errorf("unknown or inactive role %q", key) }
		seen[key] = true
		out = append(out, key)
	}
	if len(out) == 0 { return nil, errors.New("at least one role is required") }
	return out, nil
}

func (a *app) permissionsForRoles(roles []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, role := range roles {
		definition, ok := a.roleDefinitionByKey(role)
		if !ok { continue }
		for _, permission := range definition.Permissions {
			if permission == "*" { return []string{"*"} }
			if !seen[permission] { seen[permission]=true;out=append(out,permission) }
		}
	}
	sort.Strings(out)
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

func (a *app) hasPermission(u user, required string) bool {
	if strings.TrimSpace(required) == "" { return true }
	for _, permission := range a.permissionsForRoles(u.Roles) {
		if permission == "*" || permission == required { return true }
	}
	return false
}

func permissionResource(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/api/v1/dashboard/summary", path == "/api/v1/search":
		return "dashboard"
	case path == "/api/v1/audit/events":
		return "audit"
	case path == "/api/v1/notifications", strings.HasPrefix(path, "/api/v1/notifications/"):
		return "notifications"
	case path == "/api/v1/admin/roles", strings.HasPrefix(path, "/api/v1/admin/roles/"), path == "/api/v1/admin/users", strings.HasPrefix(path, "/api/v1/admin/users/"),
		path == "/api/v1/admin/secrets", strings.HasPrefix(path, "/api/v1/admin/secrets/"):
		return "administration"
	case strings.HasPrefix(path, "/api/v1/partners/") && strings.Contains(path, "/portal-users"):
		return "administration"
	case strings.HasPrefix(path, "/api/v1/partners/") && strings.Contains(path, "/modules"):
		return "catalog"
	case path == "/api/v1/modules", path == "/api/v1/module-groups", path == "/api/v1/module-commercial-matrix",
		strings.HasPrefix(path, "/api/v1/modules/"), strings.HasPrefix(path, "/api/v1/module-groups/"):
		return "catalog"
	case path == "/api/v1/partner-categories", path == "/api/v1/partners", strings.HasPrefix(path, "/api/v1/partners/"):
		return "partners"
	case strings.HasPrefix(path, "/api/v1/billing/"), strings.HasPrefix(path, "/api/v1/payments/"):
		return "billing"
	case path == "/api/v1/contact/inquiries", strings.HasPrefix(path, "/api/v1/contact/inquiries/"):
		return "contact"
	case strings.HasPrefix(path, "/api/v1/provisioning/"):
		return "provisioning"
	case path == "/api/v1/environments", strings.HasPrefix(path, "/api/v1/environments/"):
		return "environments"
	case strings.HasPrefix(path, "/api/v1/connectors/"):
		return "connectors"
	case strings.HasPrefix(path, "/api/v1/system-health"):
		return "health"
	case path == "/api/v1/backups", strings.HasPrefix(path, "/api/v1/backups/"):
		return "backups"
	case strings.HasPrefix(path, "/api/v1/impact/"):
		return "impact"
	case path == "/api/v1/evidence", strings.HasPrefix(path, "/api/v1/evidence/"):
		return "evidence"
	case path == "/api/v1/reports", strings.HasPrefix(path, "/api/v1/reports/"):
		return "reports"
	case path == "/api/v1/cms/pages", strings.HasPrefix(path, "/api/v1/cms/pages/"),
		path == "/api/v1/cms/media", strings.HasPrefix(path, "/api/v1/cms/media/"),
		path == "/api/v1/cms/design", strings.HasPrefix(path, "/api/v1/cms/design/"),
		path == "/api/v1/cms/seo", strings.HasPrefix(path, "/api/v1/cms/seo/"):
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
	case resource == "notifications":
		action = "read"
	case resource == "administration" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions:
		action = "approve"
	case resource == "cms" && (strings.HasSuffix(path, "/publish") || strings.HasSuffix(path, "/rollback")):
		action = "approve"
	case resource == "provisioning" && strings.HasSuffix(path, "/run"):
		action = "approve"
	case resource == "environments" && (strings.HasSuffix(path, "/deploy") || strings.HasSuffix(path, "/launch")):
		action = "approve"
	case resource == "backups" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions:
		action = "approve"
	case resource == "connectors" && path == "/api/v1/connectors/start22/retention" && r.Method == http.MethodPost:
		action = "approve"
	case resource == "evidence" && r.Method == http.MethodPatch:
		action = "approve"
	case resource == "billing" && strings.Contains(path,"/commercial-mode") && r.Method == http.MethodPatch:
		action = "approve"
	case resource == "billing" &&
		((r.Method == http.MethodPut && (strings.Contains(path, "/license") || strings.Contains(path, "/agreement") || strings.Contains(path, "/payments/"))) ||
		 (r.Method == http.MethodPost && strings.HasSuffix(path, "/license/collect"))):
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

func auditPartnerIDFromState(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key,item := range typed {
			if strings.EqualFold(strings.TrimSpace(key),"partner_id") {
				if id := strings.TrimSpace(fmt.Sprint(item)); strings.HasPrefix(id,"ptr_") {
					return id
				}
			}
		}
		for _,item := range typed {
			if id := auditPartnerIDFromState(item); id != "" { return id }
		}
	case []any:
		for _,item := range typed {
			if id := auditPartnerIDFromState(item); id != "" { return id }
		}
	}
	return ""
}

func auditAction(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/api/v1/cms/seo/publish" && r.Method == http.MethodPost:
		return "SEO_SETTINGS_PUBLISHED"
	case path == "/api/v1/cms/seo/draft" && r.Method == http.MethodPut:
		return "SEO_SETTINGS_DRAFT_SAVED"
	case path == "/api/v1/cms/design/publish" && r.Method == http.MethodPost:
		return "DESIGN_GUIDE_PUBLISHED"
	case path == "/api/v1/cms/design/draft" && r.Method == http.MethodPut:
		return "DESIGN_GUIDE_DRAFT_SAVED"
	case path == "/api/v1/profile" && r.Method == http.MethodPatch:
		return "PROFILE_UPDATED"
	case path == "/api/v1/profile/password" && r.Method == http.MethodPost:
		return "PROFILE_PASSWORD_CHANGED"
	case path == "/api/v1/admin/users" && r.Method == http.MethodPost:
		return "ADMIN_USER_CREATED"
	case path == "/api/v1/admin/roles" && r.Method == http.MethodPost:
		return "ADMIN_ROLE_CREATED"
	case strings.HasPrefix(path, "/api/v1/admin/roles/") && r.Method == http.MethodPatch:
		return "ADMIN_ROLE_UPDATED"
	case path == "/api/v1/modules" && r.Method == http.MethodPost:
		return "MODULE_CREATED"
	case strings.HasPrefix(path, "/api/v1/modules/") && r.Method == http.MethodPatch:
		return "MODULE_UPDATED"
	case strings.Contains(path, "/relationships") && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		return "MODULE_RELATIONSHIP_CHANGED"
	case strings.Contains(path, "/impact-metrics") && r.Method == http.MethodPut:
		return "MODULE_IMPACT_MAPPING_CHANGED"
	case strings.HasPrefix(path, "/api/v1/admin/users/") && r.Method == http.MethodPatch:
		return "ADMIN_USER_UPDATED"
	case strings.HasPrefix(path, "/api/v1/partners/") && strings.HasSuffix(path, "/logo") && r.Method == http.MethodPost:
		return "PARTNER_LOGO_UPLOADED"
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
	case strings.HasPrefix(path, "/api/v1/contact/inquiries/") && r.Method == http.MethodPatch:
		return "CONTACT_LEAD_UPDATED"
	case strings.Contains(path, "/license") && r.Method == http.MethodPut:
		return "LICENSE_CHANGED"
	case path == "/api/v1/backups" && r.Method == http.MethodPost:
		return "BACKUP_RESTORE_POINT_QUEUED"
	case strings.HasSuffix(path, "/restore-test") && r.Method == http.MethodPost:
		return "BACKUP_RESTORE_TEST_QUEUED"
	case strings.Contains(path, "/backups/policies/") && r.Method == http.MethodPut:
		return "BACKUP_POLICY_UPDATED"
	case path == "/api/v1/backups/prune" && r.Method == http.MethodPost:
		return "BACKUP_RETENTION_PRUNED"
	}
	resource, _ := auditResource(r)
	resource = strings.ToUpper(strings.ReplaceAll(resource, "-", "_"))
	if resource == "" { resource = "API" }
	return resource + "_" + strings.ToUpper(r.Method)
}

func (a *app) auditUserState(u user) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":append([]string(nil),u.Roles...),"active":u.Active,
		"system_owner":u.SystemOwner,"preferred_locale":normalizedLocale(u.PreferredLocale),"timezone":normalizedTimezone(u.Timezone),
		"job_title":u.JobTitle,"phone":u.Phone,"permissions":a.permissionsForRoles(u.Roles),
	}
}

func (a *app) auditOldState(r *http.Request) any {
	if r == nil { return map[string]any{} }
	if r.Method == http.MethodPatch && r.URL.Path == "/api/v1/profile" {
		if u, err := a.auth(r); err == nil { return a.auditUserState(u) }
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/profile/password" {
		if u, err := a.auth(r); err == nil { return a.auditUserState(u) }
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/") {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/")
		if id != "" && !strings.Contains(id, "/") {
			if u, err := a.findUser("id", id); err == nil { return a.auditUserState(u) }
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
			// Some control-plane mutations identify the partner in the JSON state
			// rather than the URL (for example provisioning jobs and environment
			// creation). Preserve tenant-scoped auditability by enriching only when
			// the route classifier did not already provide an authoritative partner.
			if partnerID == "" {
				partnerID = auditPartnerIDFromState(newState)
				if partnerID == "" { partnerID = auditPartnerIDFromState(requestState) }
			}
			event := auditEvent{
				ActorID: u.ID, ActorName: u.Name, ActorRoles: append([]string(nil), u.Roles...),
				RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
				CorrelationID: strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
				Action: auditAction(r),
				Method: r.Method, Path: r.URL.Path, Resource: resource, PartnerID: partnerID,
				Status: status, Outcome: outcome, OldState: oldState, NewState: newState,
				DurationMS: time.Since(started).Milliseconds(), CreatedAt: time.Now().UTC(),
			}
			a.enqueueAudit(event)
			if resource != "notifications" { go a.emitNotification(event) }
		}()
	}
	required := requiredPermission(r)
	if required != "" && !a.hasPermission(u, required) {
		common.APIError(w, 403, "FORBIDDEN", "Required permission: "+required)
		return
	}
	r.Header.Set("X-Himate-User-ID", u.ID)
	if r.URL.Path == "/api/v1/dashboard/summary" {
		a.dashboard(w, r, u)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/profile":
		a.profile(w,r,u)
	case r.URL.Path == "/api/v1/profile/password":
		a.profilePassword(w,r,u)
	case r.URL.Path == "/api/v1/admin/roles":
		a.adminRoles(w, r, u)
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/roles/"):
		a.adminRole(w, r, u)
	case r.URL.Path == "/api/v1/admin/users":
		a.adminUsers(w, r, u)
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/"):
		a.adminUser(w, r, u)
	case r.URL.Path == "/api/v1/admin/secrets" || strings.HasPrefix(r.URL.Path, "/api/v1/admin/secrets/"):
		a.adminSecrets(w, r, u)
	case r.URL.Path == "/api/v1/audit/events" && r.Method == http.MethodGet:
		a.auditEvents(w, r)
	case r.URL.Path == "/api/v1/search" && r.Method == http.MethodGet:
		a.globalSearch(w, r, u)
	case r.URL.Path == "/api/v1/notifications" || strings.HasPrefix(r.URL.Path, "/api/v1/notifications/"):
		r.Header.Set("X-Himate-Permissions", strings.Join(a.permissionsForRoles(u.Roles), ","))
		a.serveProxy(w, r, "notifications")
	case r.URL.Path == "/api/v1/partners/portfolio" && r.Method == http.MethodGet:
		a.partnerPortfolioMetrics(w, r)
	case r.URL.Path == "/api/v1/partners" && r.Method == http.MethodGet:
		a.partnerPortfolio(w, r)
	case r.URL.Path == "/api/v1/partners", r.URL.Path == "/api/v1/partner-categories":
		if r.URL.Path == "/api/v1/partners" && r.Method == http.MethodPost {
			if !a.requireServiceReleases(w, r, "partners", "billing", "cms", "storage") {
				return
			}
		}
		a.serveProxy(w, r, "partners")
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/") && strings.HasSuffix(r.URL.Path, "/logo"):
		if !a.requireServiceReleases(w, r, "partners", "cms", "storage") {
			return
		}
		a.adminPartnerLogo(w, r, u)
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/") && strings.Contains(r.URL.Path, "/portal-users"):
		a.adminPartnerUsers(w, r, u)
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/") && strings.Contains(r.URL.Path, "/modules"):
		a.serveProxy(w, r, "catalog")
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/"):
		a.serveProxy(w, r, "partners")
	case r.URL.Path == "/api/v1/modules", r.URL.Path == "/api/v1/module-groups", r.URL.Path == "/api/v1/module-commercial-matrix",
		strings.HasPrefix(r.URL.Path, "/api/v1/modules/"), strings.HasPrefix(r.URL.Path, "/api/v1/module-groups/"):
		a.serveProxy(w, r, "catalog")
	case strings.HasPrefix(r.URL.Path, "/api/v1/billing/"):
		a.serveProxy(w, r, "billing")
	case strings.HasPrefix(r.URL.Path, "/api/v1/payments/"):
		a.serveProxy(w, r, "payments")
	case r.URL.Path == "/api/v1/contact/inquiries", strings.HasPrefix(r.URL.Path, "/api/v1/contact/inquiries/"):
		a.serveProxy(w, r, "contact")
	case strings.HasPrefix(r.URL.Path, "/api/v1/provisioning/"):
		a.serveProxy(w, r, "provisioning")
	case r.URL.Path == "/api/v1/environments", strings.HasPrefix(r.URL.Path, "/api/v1/environments/"):
		a.serveProxy(w, r, "environments")
	case strings.HasPrefix(r.URL.Path, "/api/v1/connectors/"):
		a.serveProxy(w, r, "connector")
	case strings.HasPrefix(r.URL.Path, "/api/v1/system-health"):
		a.serveProxy(w, r, "health")
	case r.URL.Path == "/api/v1/backups", strings.HasPrefix(r.URL.Path, "/api/v1/backups/"):
		a.serveProxy(w, r, "backups")
	case strings.HasPrefix(r.URL.Path, "/api/v1/impact/"):
		a.serveProxy(w, r, "impact")
	case r.URL.Path == "/api/v1/evidence", strings.HasPrefix(r.URL.Path, "/api/v1/evidence/"):
		a.serveProxy(w, r, "evidence")
	case r.URL.Path == "/api/v1/reports", strings.HasPrefix(r.URL.Path, "/api/v1/reports/"):
		a.serveProxy(w, r, "reports")
	case r.URL.Path == "/api/v1/cms/pages", strings.HasPrefix(r.URL.Path, "/api/v1/cms/pages/"),
		r.URL.Path == "/api/v1/cms/media", strings.HasPrefix(r.URL.Path, "/api/v1/cms/media/"),
		r.URL.Path == "/api/v1/cms/design", strings.HasPrefix(r.URL.Path, "/api/v1/cms/design/"),
		r.URL.Path == "/api/v1/cms/seo", strings.HasPrefix(r.URL.Path, "/api/v1/cms/seo/"):
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
	case "payments":
		resource = "billing"
		if len(parts) > 2 && parts[1] == "partners" { partnerID = parts[2] }
	case "connectors":
		resource = "connectors"
		if len(parts) > 1 { partnerID = parts[1] }
	case "cms":
		resource = "cms"
	case "contact":
		resource = "contact"
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
	case "backups":
		resource = "backups"
		if len(parts) > 2 && parts[1] == "policies" { partnerID = parts[2] }
	case "modules", "module-groups", "module-commercial-matrix":
		resource = "catalog"
	case "notifications":
		resource = "notifications"
	case "admin":
		resource = "administration"
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

func notificationDescriptor(event auditEvent) (severity,title,message,audience,deepLink string,boolValue bool) {
	if event.Outcome=="FAILED" {
		switch event.Resource {
		case "environments","backups","connectors","provisioning":
			return "WARNING", strings.ReplaceAll(strings.Title(strings.ToLower(event.Action)),"_"," "), "A protected operation failed. Review the audit trail for details.", event.Resource+".read", "/app", true
		default:
			return "","","","","",false
		}
	}
	switch event.Action {
	case "ADMIN_USER_CREATED": return "INFO","Administrator created",event.ActorName+" created a HIMATE administrator.","administration.read","/app",true
	case "ADMIN_USER_UPDATED": return "INFO","Administrator access changed",event.ActorName+" updated administrator access.","administration.read","/app",true
	case "ADMIN_ROLE_CREATED": return "INFO","Custom role created",event.ActorName+" created a custom access role.","administration.read","/app",true
	case "ADMIN_ROLE_UPDATED": return "INFO","Custom role updated",event.ActorName+" changed a custom access role.","administration.read","/app",true
	case "MODULE_CREATED": return "INFO","Module created",event.ActorName+" added a module to the HIMATE registry.","catalog.read","/app",true
	case "MODULE_UPDATED","MODULE_RELATIONSHIP_CHANGED","MODULE_IMPACT_MAPPING_CHANGED":
		return "INFO","Module registry changed",event.ActorName+" updated module control-plane configuration.","catalog.read","/app",true
	case "LICENSE_CHANGED": return "INFO","Commercial terms changed","Partner licensing/payment state was updated.","billing.read","/app",true
	case "ENVIRONMENT_DEPLOY": return "INFO","Deployment started","A partner environment deployment was requested.","environments.read","/app",true
	case "ENVIRONMENT_LAUNCH": return "INFO","Production launch requested","A partner production launch was requested.","environments.read","/app",true
	case "BACKUP_RESTORE_POINT_QUEUED","BACKUP_RESTORE_TEST_QUEUED": return "INFO","Backup operation queued","A recoverability operation was queued.","backups.read","/app",true
	case "SEO_SETTINGS_PUBLISHED": return "INFO","SEO settings published","Published SEO settings changed.","cms.read","/app",true
	}
	if event.Resource=="partners" && event.Method==http.MethodPost { return "INFO","Partner created","A new partner record was registered.","partners.read","/app",true }
	return "","","","","",false
}

func (a *app) emitNotification(event auditEvent) {
	host:=strings.TrimSpace(a.hosts["notifications"]);if host==""{return}
	severity,title,message,audience,deepLink,ok:=notificationDescriptor(event);if !ok{return}
	body,_:=json.Marshal(map[string]any{
		"event_type":event.Action,"severity":severity,"title":title,"message":message,"resource":event.Resource,
		"partner_id":event.PartnerID,"deep_link":deepLink,"audience_permission":audience,
		"delivery_scope":"PLATFORM","category":"SYSTEM",
		"metadata":map[string]any{"actor_id":event.ActorID,"actor_name":event.ActorName,"request_id":event.RequestID,"correlation_id":event.CorrelationID},
	})
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel()
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+host+"/internal/v1/notifications/events",bytes.NewReader(body));if err!=nil{return}
	req.Header.Set("Content-Type","application/json")
	common.BindInternalRequest(req,a.internalToken)
	resp,err:=common.DoInternal(a.client,req);if err==nil&&resp!=nil{resp.Body.Close()}
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
	serviceVersions := map[string]string{"gateway": a.version}
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
			version := strings.TrimSpace(resp.Header.Get("X-Himate-App-Version"))
			serviceVersions[name] = version
			switch {
			case version == "":
				services[name] = "version_unknown"
				overall = "degraded"
			case version != a.version:
				services[name] = "version_mismatch"
				overall = "degraded"
			default:
				services[name] = "ok"
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	common.JSON(w, 200, map[string]any{
		"status": overall,
		"service": "himate-gateway",
		"environment": a.env,
		"version": a.version,
		"architecture": "containerized-microservices-start-23.11.3k",
		"checked_at": checkedAt,
		"services": services,
		"service_versions": serviceVersions,
		"release_consistent": overall == "ok",
	})
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

func (a *app) dashboardRecentActivity(actor user, limit int) ([]map[string]any, error) {
	if limit < 1 { limit = 1 }
	if limit > 12 { limit = 12 }
	rows, err := a.db.Query(`SELECT id,actor_name,action,resource,partner_id,outcome,created_at
		FROM identity.audit_events
		WHERE outcome='SUCCESS'
		ORDER BY created_at DESC,id DESC
		LIMIT 120`)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]map[string]any,0,limit)
	for rows.Next() {
		var id int64
		var actorName,action,resource,partnerID,outcome string
		var created time.Time
		if err := rows.Scan(&id,&actorName,&action,&resource,&partnerID,&outcome,&created); err != nil { return nil, err }
		permissionResource := resource
		switch resource {
		case "modules","module-groups","module-commercial-matrix":
			permissionResource = "catalog"
		case "payments":
			permissionResource = "billing"
		case "partner-categories":
			permissionResource = "partners"
		case "admin":
			permissionResource = "administration"
		}
		if permissionResource == "" || !a.hasPermission(actor, permissionResource+".read") { continue }
		items = append(items,map[string]any{
			"id":id,"actor_name":actorName,"action":action,"resource":resource,
			"partner_id":partnerID,"outcome":outcome,"created_at":created.UTC(),
		})
		if len(items) >= limit { break }
	}
	if err := rows.Err(); err != nil { return nil, err }
	return items,nil
}

func dashboardYear(r *http.Request) (int,error) {
	year:=time.Now().UTC().Year()
	raw:=strings.TrimSpace(r.URL.Query().Get("year"))
	if raw=="" { return year,nil }
	parsed,err:=strconv.Atoi(raw)
	if err!=nil||parsed<2000||parsed>2100 { return 0,fmt.Errorf("year must be between 2000 and 2100") }
	return parsed,nil
}

func copyDashboardPayload(source map[string]any) map[string]any {
	out:=make(map[string]any,len(source)+1)
	for key,value:=range source { out[key]=value }
	return out
}

func (a *app) dashboardPayloadForActor(source map[string]any, actor user) map[string]any {
	out:=copyDashboardPayload(source)
	if a.hasPermission(actor,"billing.read") {
		if raw,ok:=source["billing"].(map[string]any);ok {
			billing:=copyDashboardPayload(raw)
			billing["authorized"]=true
			out["billing"]=billing
		}
	} else {
		out["billing"]=map[string]any{
			"authorized":false,"items":[]any{},"count":0,"status":"restricted",
		}
	}
	if a.hasPermission(actor,"impact.read") {
		if raw,ok:=source["impact"].(map[string]any);ok {
			impact:=copyDashboardPayload(raw)
			impact["authorized"]=true
			out["impact"]=impact
		}
	} else {
		out["impact"]=map[string]any{
			"authorized":false,"people_reached_ytd":nil,"trend":[]any{},"status":"restricted",
		}
	}
	return out
}

func (a *app) dashboard(w http.ResponseWriter, r *http.Request, actor user) {
	year,err:=dashboardYear(r)
	if err!=nil { common.APIError(w,http.StatusBadRequest,"VALIDATION",err.Error());return }

	activity,activityErr:=a.dashboardRecentActivity(actor,6)
	refresh:=strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")),"true")
	cacheable:=year==time.Now().UTC().Year() && !refresh

	// The shared cache intentionally excludes permission-scoped recent activity.
	// This prevents one administrator's visible audit domains from leaking to another.
	a.dashboardMu.RLock()
	if cacheable && a.dashboardPayload != nil && time.Now().Before(a.dashboardExpires) {
		payload:=a.dashboardPayloadForActor(a.dashboardPayload,actor)
		a.dashboardMu.RUnlock()
		payload["activity"]=map[string]any{"items":activity,"count":len(activity),"source":"IDENTITY_APPEND_ONLY_AUDIT"}
		if activityErr!=nil { payload["activity"]=map[string]any{"items":[]any{},"count":0,"source":"IDENTITY_APPEND_ONLY_AUDIT","status":"degraded"} }
		w.Header().Set("X-Himate-Cache","hit")
		common.JSON(w,http.StatusOK,payload)
		return
	}
	stale:=a.dashboardPayload
	a.dashboardMu.RUnlock()

	started:=time.Now()
	ctx,cancel:=context.WithTimeout(r.Context(),3*time.Second)
	defer cancel()

	var partnerResponse struct {
		Items []map[string]any `json:"items"`
		Total int `json:"total"`
		LifecycleCounts map[string]int `json:"lifecycle_counts"`
	}
	var moduleResponse struct {
		Items []map[string]any `json:"items"`
		Count int `json:"count"`
	}
	var billingResponse map[string]any
	var impactResponse map[string]any
	var partnerErr,moduleErr,billingErr,impactErr error
	var wg sync.WaitGroup
	wg.Add(4)
	go func(){ defer wg.Done(); partnerErr=a.internalGET(ctx,a.hosts["partners"],"/api/v1/partners?limit=1&offset=0&include_archived=true",&partnerResponse) }()
	go func(){ defer wg.Done(); moduleErr=a.internalGET(ctx,a.hosts["catalog"],"/api/v1/modules",&moduleResponse) }()
	go func(){ defer wg.Done(); billingErr=a.internalGET(ctx,a.hosts["billing"],fmt.Sprintf("/internal/v1/analytics/dashboard?year=%d",year),&billingResponse) }()
	go func(){ defer wg.Done(); impactErr=a.internalGET(ctx,a.hosts["impact"],fmt.Sprintf("/internal/v1/impact/dashboard?year=%d",year),&impactResponse) }()
	wg.Wait()

	if cacheable && partnerErr!=nil && moduleErr!=nil && billingErr!=nil && impactErr!=nil && stale!=nil {
		payload:=a.dashboardPayloadForActor(stale,actor)
		payload["activity"]=map[string]any{"items":activity,"count":len(activity),"source":"IDENTITY_APPEND_ONLY_AUDIT"}
		w.Header().Set("X-Himate-Cache","stale")
		w.Header().Set("Server-Timing",fmt.Sprintf("dashboard;dur=%d",time.Since(started).Milliseconds()))
		common.JSON(w,http.StatusOK,payload)
		return
	}

	live:=partnerResponse.LifecycleCounts["LIVE"]
	status:="healthy"
	if partnerErr!=nil||moduleErr!=nil||billingErr!=nil||impactErr!=nil||activityErr!=nil { status="degraded" }
	if billingResponse==nil { billingResponse=map[string]any{"year":year,"items":[]any{},"count":0,"source":"BILLING_PAID_LEDGER","status":"degraded"} }
	if impactResponse==nil {
		trend:=make([]map[string]any,0,12)
		labels:=[]string{"Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"}
		for month:=1;month<=12;month++ { trend=append(trend,map[string]any{"month":month,"label":labels[month-1],"value":0}) }
		impactResponse=map[string]any{"year":year,"metric_key":"klavierhaus.events.attendance.attendee_count","people_reached_ytd":0,"trend":trend,"source":"IMPACT_METRIC_VALUES","status":"degraded"}
	}
	core:=map[string]any{
		"year":year,
		"partners":map[string]any{"total":partnerResponse.Total,"live":live,"lifecycle_counts":partnerResponse.LifecycleCounts},
		"modules":map[string]any{"catalog_total":moduleResponse.Count},
		"billing":billingResponse,
		"impact":impactResponse,
		"system":map[string]any{"status":status,"environment":a.env,"version":a.version,"architecture":"containerized-microservices-start-23.9"},
	}
	if cacheable {
		a.dashboardMu.Lock()
		a.dashboardPayload=core
		a.dashboardExpires=time.Now().Add(10*time.Second)
		a.dashboardMu.Unlock()
	}

	payload:=a.dashboardPayloadForActor(core,actor)
	payload["activity"]=map[string]any{"items":activity,"count":len(activity),"source":"IDENTITY_APPEND_ONLY_AUDIT"}
	w.Header().Set("X-Himate-Cache","miss")
	w.Header().Set("Server-Timing",fmt.Sprintf("dashboard;dur=%d",time.Since(started).Milliseconds()))
	common.JSON(w,http.StatusOK,payload)
}

func searchText(values ...any) string {
	parts:=make([]string,0,len(values))
	for _,value:=range values { parts=append(parts,strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))) }
	return strings.Join(parts," ")
}

func searchContains(q string, values ...any) bool {
	return strings.Contains(searchText(values...),strings.ToLower(strings.TrimSpace(q)))
}

func (a *app) globalSearch(w http.ResponseWriter,r *http.Request,actor user) {
	q:=strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q))<2 { common.APIError(w,http.StatusBadRequest,"VALIDATION","Search query must contain at least 2 characters");return }
	if len([]rune(q))>100 { common.APIError(w,http.StatusBadRequest,"VALIDATION","Search query is too long");return }
	limit:=auditLimit(r.URL.Query().Get("limit"),5,10)
	ctx,cancel:=context.WithTimeout(r.Context(),3*time.Second)
	defer cancel()

	results:=[]map[string]any{}
	appendResult:=func(resource,id,title,subtitle,deepLink string) {
		results=append(results,map[string]any{
			"resource":resource,"id":id,"title":title,"subtitle":subtitle,"deep_link":deepLink,
		})
	}

	if a.hasPermission(actor,"partners.read") {
		var response struct{ Items []map[string]any `json:"items"` }
		path:="/api/v1/partners?limit="+strconv.Itoa(limit)+"&offset=0&core_only=true&include_stats=false&q="+url.QueryEscape(q)
		if a.internalGET(ctx,a.hosts["partners"],path,&response)==nil {
			for _,item:=range response.Items {
				id:=strings.TrimSpace(fmt.Sprint(item["id"]))
				name:=strings.TrimSpace(fmt.Sprint(item["name"]))
				if name=="" { name=id }
				appendResult("partners",id,name,strings.TrimSpace(fmt.Sprint(item["lifecycle"])), "/app/partners/"+url.PathEscape(id))
			}
		}
	}

	if a.hasPermission(actor,"catalog.read") {
		var response struct{ Items []map[string]any `json:"items"` }
		if a.internalGET(ctx,a.hosts["catalog"],"/api/v1/modules",&response)==nil {
			count:=0
			for _,item:=range response.Items {
				if !searchContains(q,item["key"],item["label"],item["label_en"],item["label_hu"],item["description"],item["description_en"],item["description_hu"]) { continue }
				id:=strings.TrimSpace(fmt.Sprint(item["key"]))
				title:=strings.TrimSpace(fmt.Sprint(item["label"]))
				if title=="" { title=id }
				appendResult("catalog",id,title,"Module · "+id,"/app")
				count++;if count>=limit { break }
			}
		}
	}

	if a.hasPermission(actor,"contact.read") {
		var response struct{ Items []map[string]any `json:"items"` }
		path:="/api/v1/contact/inquiries?limit="+strconv.Itoa(limit)+"&offset=0&q="+url.QueryEscape(q)
		if a.internalGET(ctx,a.hosts["contact"],path,&response)==nil {
			for _,item:=range response.Items {
				id:=strings.TrimSpace(fmt.Sprint(item["id"]))
				title:=strings.TrimSpace(fmt.Sprint(item["name"]))
				subtitle:=strings.TrimSpace(fmt.Sprint(item["organization"]))
				if subtitle=="" { subtitle=strings.TrimSpace(fmt.Sprint(item["email"])) }
				appendResult("contact",id,title,subtitle,"/app")
			}
		}
	}

	if a.hasPermission(actor,"cms.read") {
		var response struct{ Items []map[string]any `json:"items"` }
		if a.internalGET(ctx,a.hosts["cms"],"/api/v1/cms/pages",&response)==nil {
			count:=0
			for _,item:=range response.Items {
				if !searchContains(q,item["id"],item["page_key"],item["name"],item["locale"]) { continue }
				id:=strings.TrimSpace(fmt.Sprint(item["id"]))
				title:=strings.TrimSpace(fmt.Sprint(item["name"]))
				appendResult("cms",id,title,"CMS · "+strings.TrimSpace(fmt.Sprint(item["locale"])),"/app")
				count++;if count>=limit { break }
			}
		}
	}

	if a.hasPermission(actor,"administration.read") {
		like:="%"+q+"%"
		rows,err:=a.db.QueryContext(ctx,`SELECT id,name,email FROM identity.users
			WHERE name ILIKE $1 OR email ILIKE $1 ORDER BY system_owner DESC,active DESC,lower(name) LIMIT $2`,like,limit)
		if err==nil {
			for rows.Next() {
				var id,name,email string
				if rows.Scan(&id,&name,&email)==nil { appendResult("administration",id,name,email,"/app") }
			}
			rows.Close()
		}
	}

	if a.hasPermission(actor,"audit.read") {
		like:="%"+q+"%"
		rows,err:=a.db.QueryContext(ctx,`SELECT id,action,actor_name,resource,created_at FROM identity.audit_events
			WHERE action ILIKE $1 OR actor_name ILIKE $1 OR resource ILIKE $1 OR partner_id ILIKE $1
			ORDER BY created_at DESC,id DESC LIMIT $2`,like,limit)
		if err==nil {
			for rows.Next() {
				var id int64;var action,actorName,resource string;var created time.Time
				if rows.Scan(&id,&action,&actorName,&resource,&created)==nil {
					appendResult("audit",strconv.FormatInt(id,10),strings.ReplaceAll(action,"_"," "),actorName+" · "+resource,"/app")
				}
			}
			rows.Close()
		}
	}

	common.JSON(w,http.StatusOK,map[string]any{
		"query":q,"items":results,"count":len(results),"limit_per_resource":limit,
		"permission_scoped":true,
	})
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
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (a *app) serviceRelease(ctx context.Context, service string) (string, error) {
	host := strings.TrimSpace(a.hosts[service])
	if host == "" {
		return "", fmt.Errorf("%s service host is not configured", service)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/health", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Himate-Internal-Token", a.internalToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s health returned status %d", service, resp.StatusCode)
	}
	version := strings.TrimSpace(resp.Header.Get("X-Himate-App-Version"))
	if version == "" {
		return "", fmt.Errorf("%s does not report X-Himate-App-Version", service)
	}
	if version != a.version {
		return version, fmt.Errorf("%s is running %s while gateway requires %s", service, version, a.version)
	}
	return version, nil
}

func (a *app) requireServiceReleases(w http.ResponseWriter, r *http.Request, services ...string) bool {
	type result struct {
		service string
		version string
		err     error
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2500*time.Millisecond)
	defer cancel()
	results := make(chan result, len(services))
	for _, service := range services {
		service := service
		go func() {
			version, err := a.serviceRelease(ctx, service)
			results <- result{service: service, version: version, err: err}
		}()
	}
	failures := make([]string, 0)
	for range services {
		item := <-results
		if item.err != nil {
			failures = append(failures, item.err.Error())
		}
	}
	if len(failures) > 0 {
		common.APIError(w, http.StatusServiceUnavailable, "RELEASE_MISMATCH",
			"Operation blocked because microservice releases are not synchronized: "+strings.Join(failures, "; "))
		return false
	}
	return true
}

func (a *app) serveProxy(w http.ResponseWriter, r *http.Request, service string) {
	proxy := a.proxies[service]
	if proxy == nil {
		common.APIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", service+" service is temporarily unavailable")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		if !a.requireServiceReleases(w, r, service) {
			return
		}
	}
	proxy.ServeHTTP(w, r)
}

func newProxy(host, token, expectedVersion string) (*httputil.ReverseProxy, error) {
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("private service host is required")
	}
	target, err := url.Parse("http://" + host)
	if err != nil {
		return nil, err
	}
	p := httputil.NewSingleHostReverseProxy(target)
	base := p.Director
	p.Director = func(r *http.Request) {
		base(r)
		r.Header.Set("X-Himate-Internal-Token", token)
		r.Header.Set("X-Himate-Expected-Version", expectedVersion)
	}
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
		common.JSON(w,http.StatusOK,a.publicUser(actor))
	case http.MethodPatch:
		var in struct {
			Name *string `json:"name"`
			Email *string `json:"email"`
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
		if in.Email!=nil {
			next.Email=strings.ToLower(strings.TrimSpace(*in.Email))
			if !validEmail(next.Email) { common.APIError(w,400,"VALIDATION","A valid email is required");return }
			var duplicate bool
			_ = a.db.QueryRow(`SELECT
				EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1) AND id<>$2)
				OR EXISTS(SELECT 1 FROM identity.partner_users WHERE lower(email)=lower($1))`,next.Email,actor.ID).Scan(&duplicate)
			if duplicate { common.APIError(w,409,"EMAIL_EXISTS","This email already belongs to another HIMATE or Partner Portal identity");return }
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
		_,err:=a.db.Exec(`UPDATE identity.users SET name=$2,email=$3,preferred_locale=$4,timezone=$5,job_title=$6,phone=$7,updated_at=NOW() WHERE id=$1`,
			actor.ID,next.Name,next.Email,next.PreferredLocale,next.Timezone,next.JobTitle,next.Phone)
		if err!=nil { common.APIError(w,500,"DB","Could not update profile");return }
		common.JSON(w,http.StatusOK,a.publicUser(next))
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
	if message:=passwordPolicyError(in.NewPassword);message!="" {
		common.APIError(w,400,"VALIDATION",message);return
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
	common.JSON(w,http.StatusOK,a.publicUser(next))
}

func (a *app) adminUserMap(u user, createdAt, updatedAt time.Time) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":u.Roles,"active":u.Active,
		"system_owner":u.SystemOwner,"preferred_locale":normalizedLocale(u.PreferredLocale),
		"timezone":normalizedTimezone(u.Timezone),"job_title":u.JobTitle,"phone":u.Phone,
		"permissions":a.permissionsForRoles(u.Roles),
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

func passwordPolicyError(password string) string {
	if len([]rune(password)) < 12 {
		return "Password must be at least 12 characters and include lowercase, uppercase, a number and a special character"
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
		return "Password must be at least 12 characters and include lowercase, uppercase, a number and a special character"
	}
	return ""
}

func (a *app) adminRoles(w http.ResponseWriter, r *http.Request, actor user) {
	locale:=common.RequestLocale(r)
	switch r.Method {
	case http.MethodGet:
		items := make([]map[string]any,0,len(roleDefinitions)+8)
		for _,role := range roleDefinitions {
			items=append(items,map[string]any{
				"key":role.Key,"label":role.Label,"label_en":role.Label,"label_hu":role.Label,
				"description":role.Description,"description_en":role.Description,"description_hu":role.Description,
				"permissions":role.Permissions,"system":true,"active":true,
			})
		}
		rows,err:=a.db.Query(`SELECT role_key,label_en,label_hu,description_en,description_hu,permissions,active,created_by,created_at,updated_at
			FROM identity.custom_roles ORDER BY active DESC,lower(label_en),role_key`)
		if err!=nil{common.APIError(w,500,"DB","Could not load custom roles");return}
		defer rows.Close()
		for rows.Next(){
			var key,labelEN,labelHU,descEN,descHU,createdBy string;var raw []byte;var active bool;var created,updated time.Time
			if rows.Scan(&key,&labelEN,&labelHU,&descEN,&descHU,&raw,&active,&createdBy,&created,&updated)==nil{
				permissions:=[]string{};_ = json.Unmarshal(raw,&permissions)
				items=append(items,map[string]any{
					"key":key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
					"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
					"permissions":permissions,"system":false,"active":active,"created_by":createdBy,"created_at":created,"updated_at":updated,
				})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items),"locale":locale})
	case http.MethodPost:
		if !ownerRequired(w,actor){return}
		var in struct{
			Key string `json:"key"`
			Label string `json:"label"`
			LabelEN string `json:"label_en"`
			LabelHU string `json:"label_hu"`
			Description string `json:"description"`
			DescriptionEN string `json:"description_en"`
			DescriptionHU string `json:"description_hu"`
			Permissions []string `json:"permissions"`
		}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		in.Key=strings.ToLower(strings.TrimSpace(in.Key))
		labelEN:=strings.TrimSpace(in.LabelEN);labelHU:=strings.TrimSpace(in.LabelHU);legacyLabel:=strings.TrimSpace(in.Label)
		if labelEN==""{labelEN=legacyLabel};if labelHU==""{labelHU=legacyLabel}
		descEN:=strings.TrimSpace(in.DescriptionEN);descHU:=strings.TrimSpace(in.DescriptionHU);legacyDesc:=strings.TrimSpace(in.Description)
		if descEN==""{descEN=legacyDesc};if descHU==""{descHU=legacyDesc}
		if !roleKeyPattern.MatchString(in.Key)||labelEN==""||labelHU==""{common.APIError(w,400,"VALIDATION","Stable role key and bilingual labels are required");return}
		if _,exists:=builtinRoleDefinitionByKey(in.Key);exists{common.APIError(w,409,"ROLE_RESERVED","Built-in role keys are reserved");return}
		permissions,err:=normalizeCustomPermissions(in.Permissions);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
		raw,_:=json.Marshal(permissions)
		if _,err=a.db.Exec(`INSERT INTO identity.custom_roles(role_key,label,label_en,label_hu,description,description_en,description_hu,permissions,created_by)
			VALUES($1,$2,$2,$3,$4,$4,$5,$6::jsonb,$7)`,
			in.Key,labelEN,labelHU,descEN,descHU,string(raw),actor.ID);err!=nil{common.APIError(w,409,"CONFLICT","Role could not be created");return}
		common.JSON(w,201,map[string]any{
			"key":in.Key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
			"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
			"permissions":permissions,"system":false,"active":true,
		})
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) adminRole(w http.ResponseWriter,r *http.Request,actor user){
	if !ownerRequired(w,actor){return}
	if r.Method!=http.MethodPatch{common.APIError(w,405,"METHOD","Use PATCH");return}
	key:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/admin/roles/"),"/")
	if key==""||strings.Contains(key,"/"){common.APIError(w,404,"NOT_FOUND","Role not found");return}
	if _,exists:=builtinRoleDefinitionByKey(key);exists{common.APIError(w,409,"ROLE_PROTECTED","Built-in roles cannot be modified");return}
	locale:=common.RequestLocale(r)
	var labelEN,labelHU,descEN,descHU string;var raw []byte;var active bool
	if err:=a.db.QueryRow(`SELECT label_en,label_hu,description_en,description_hu,permissions,active FROM identity.custom_roles WHERE role_key=$1`,key).
		Scan(&labelEN,&labelHU,&descEN,&descHU,&raw,&active);err!=nil{
		common.APIError(w,404,"NOT_FOUND","Role not found");return
	}
	permissions:=[]string{};_ = json.Unmarshal(raw,&permissions)
	var in struct{
		Label *string `json:"label"`;LabelEN *string `json:"label_en"`;LabelHU *string `json:"label_hu"`
		Description *string `json:"description"`;DescriptionEN *string `json:"description_en"`;DescriptionHU *string `json:"description_hu"`
		Permissions *[]string `json:"permissions"`;Active *bool `json:"active"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	if in.Label!=nil{legacy:=strings.TrimSpace(*in.Label);if in.LabelEN==nil{labelEN=legacy};if in.LabelHU==nil{labelHU=legacy}}
	if in.LabelEN!=nil{labelEN=strings.TrimSpace(*in.LabelEN)}
	if in.LabelHU!=nil{labelHU=strings.TrimSpace(*in.LabelHU)}
	if in.Description!=nil{legacy:=strings.TrimSpace(*in.Description);if in.DescriptionEN==nil{descEN=legacy};if in.DescriptionHU==nil{descHU=legacy}}
	if in.DescriptionEN!=nil{descEN=strings.TrimSpace(*in.DescriptionEN)}
	if in.DescriptionHU!=nil{descHU=strings.TrimSpace(*in.DescriptionHU)}
	if in.Active!=nil{active=*in.Active}
	if in.Permissions!=nil{next,err:=normalizeCustomPermissions(*in.Permissions);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return};permissions=next}
	if labelEN==""||labelHU==""{common.APIError(w,400,"VALIDATION","English and Hungarian role labels are required");return}
	if !active{
		var assigned int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM identity.users WHERE roles ? $1`,key).Scan(&assigned)
		if assigned>0{common.APIError(w,409,"ROLE_IN_USE","Remove this role from administrators before deactivating it");return}
	}
	raw,_=json.Marshal(permissions)
	if _,err:=a.db.Exec(`UPDATE identity.custom_roles SET label=$2,label_en=$2,label_hu=$3,description=$4,description_en=$4,description_hu=$5,permissions=$6::jsonb,active=$7,updated_at=NOW() WHERE role_key=$1`,
		key,labelEN,labelHU,descEN,descHU,string(raw),active);err!=nil{common.APIError(w,500,"DB","Could not update role");return}
	common.JSON(w,200,map[string]any{
		"key":key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
		"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
		"permissions":permissions,"system":false,"active":active,
	})
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
			items = append(items, a.adminUserMap(u,createdAt,updatedAt))
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
		if message:=passwordPolicyError(in.Password); message!="" { common.APIError(w,400,"VALIDATION",message); return }
		roles, err := a.normalizeRoles(in.Roles)
		if err != nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
		if containsRole(roles,"platform_admin") { common.APIError(w,409,"OWNER_ROLE_RESERVED","Platform Admin is reserved for the HIMATE system owner"); return }
		var exists bool
		_ = a.db.QueryRow(`SELECT
			EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1))
			OR EXISTS(SELECT 1 FROM identity.partner_users WHERE lower(email)=lower($1))`,in.Email).Scan(&exists)
		if exists { common.APIError(w,409,"EMAIL_EXISTS","This email already belongs to another HIMATE or Partner Portal identity"); return }
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
		common.JSON(w,201,a.adminUserMap(u,createdAt,updatedAt))
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
		_ = a.db.QueryRow(`SELECT
			EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1) AND id<>$2)
			OR EXISTS(SELECT 1 FROM identity.partner_users WHERE lower(email)=lower($1))`,next.Email,id).Scan(&duplicate)
		if duplicate { common.APIError(w,409,"EMAIL_EXISTS","This email already belongs to another HIMATE or Partner Portal identity"); return }
	}
	if in.Roles != nil {
		next.Roles, err = a.normalizeRoles(*in.Roles)
		if err != nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
		if !current.SystemOwner && containsRole(next.Roles,"platform_admin") { common.APIError(w,409,"OWNER_ROLE_RESERVED","Platform Admin is reserved for the HIMATE system owner"); return }
	}
	if in.Active != nil { next.Active = *in.Active }

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
	if current.SystemOwner && (!next.Active || !containsRole(next.Roles,"platform_admin")) {
		common.APIError(w,409,"OWNER_PROTECTED","The HIMATE system owner must remain an active Platform Admin")
		return
	}

	hash := current.PasswordHash
	sessionVersion := current.SessionVersion
	passwordChanged := false
	authChanged := in.Email != nil || in.Roles != nil || in.Active != nil
	if in.Password != nil {
		if message:=passwordPolicyError(*in.Password); message!="" { common.APIError(w,400,"VALIDATION",message); return }
		hash, err = hashPassword(*in.Password)
		if err != nil { common.APIError(w,500,"PASSWORD","Could not secure password"); return }
		passwordChanged = true
		authChanged = true
	}
	if authChanged { sessionVersion++ }
	rolesRaw, _ := json.Marshal(next.Roles)
	var createdAt, updatedAt time.Time
	err = a.db.QueryRow(`UPDATE identity.users SET name=$2,email=$3,password_hash=$4,roles=$5::jsonb,active=$6,
			session_version=$7,password_changed_at=CASE WHEN $8 THEN NOW() ELSE password_changed_at END,updated_at=NOW()
		WHERE id=$1 RETURNING created_at,updated_at`,id,next.Name,next.Email,hash,string(rolesRaw),next.Active,sessionVersion,passwordChanged).Scan(&createdAt,&updatedAt)
	if err != nil { common.APIError(w,500,"DB","Could not update administration user"); return }
	next.PasswordHash = hash
	next.SessionVersion = sessionVersion
	common.JSON(w,200,a.adminUserMap(next,createdAt,updatedAt))
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
func (a *app) publicUser(u user) map[string]any {
	return map[string]any{
		"id":u.ID,"name":u.Name,"email":u.Email,"roles":u.Roles,
		"permissions":a.permissionsForRoles(u.Roles),
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
	Title           string         `json:"title"`
	MetaDescription string         `json:"meta_description"`
	Keywords        []string       `json:"keywords"`
	JSONLD          map[string]any `json:"json_ld"`
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
	Locale         string             `json:"locale"`
	Slug           string             `json:"slug"`
	SEO            publicCMSSEO       `json:"seo"`
	Sections       []publicCMSSection `json:"sections"`
	HiddenSections []string           `json:"hidden_sections"`
	Alternates     map[string]string  `json:"alternates"`
}

type publicSEOSettings struct {
	Locale                string   `json:"locale"`
	Version               int      `json:"version"`
	GlobalKeywords        []string `json:"global_keywords"`
	OrganizationName      string   `json:"organization_name"`
	OrganizationURL       string   `json:"organization_url"`
	DefaultOGImageAssetID string   `json:"default_og_image_asset_id"`
}

type publicNavigationItem struct {
	LabelEN   string `json:"label_en"`
	LabelHU   string `json:"label_hu"`
	URL       string `json:"url"`
	Visible   bool   `json:"visible"`
	SortOrder int    `json:"sort_order"`
}

type publicSiteDesign struct {
	LogoMediaAssetID string                 `json:"logo_media_asset_id"`
	Assets           map[string]string      `json:"assets"`
	LayoutKey        string                 `json:"layout_key"`
	Navy             string                 `json:"navy"`
	Gold             string                 `json:"gold"`
	Background       string                 `json:"background"`
	TextColor        string                 `json:"text_color"`
	HeadingFont      string                 `json:"heading_font"`
	BodyFont         string                 `json:"body_font"`
	ButtonRadius     int                    `json:"button_radius"`
	Navigation       []publicNavigationItem `json:"navigation"`
}

type publicSiteDesignEnvelope struct {
	Version int              `json:"version"`
	Design  publicSiteDesign `json:"design"`
}

type publicCMSManifest struct {
	Items []struct {
		Slug      string `json:"slug"`
		Locale    string `json:"locale"`
		Canonical string `json:"canonical"`
		Title     string `json:"title"`
		NoIndex   bool   `json:"noindex"`
	} `json:"items"`
}

func normalizePublicLocale(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "hu" || value == "hu_hu" || strings.HasPrefix(value, "hu-") {
		return "hu_HU"
	}
	return "en_US"
}

func publicLocale(r *http.Request) string {
	if r == nil { return "en_US" }
	if raw := strings.TrimSpace(r.URL.Query().Get("lang")); raw != "" {
		return normalizePublicLocale(raw)
	}
	if cookie, err := r.Cookie("himate_public_locale"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return normalizePublicLocale(cookie.Value)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept-Language"))), "hu") {
		return "hu_HU"
	}
	return "en_US"
}

func (a *app) fetchPublishedCMS(ctx context.Context, slug, locale string) (publicCMSPage, error) {
	var out publicCMSPage
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	endpoint := "http://"+host+"/public/v1/cms/pages/"+url.PathEscape(slug)+"?locale="+url.QueryEscape(normalizePublicLocale(locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
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

func (a *app) fetchPreviewCMS(ctx context.Context, slug, token string) (publicCMSPage, error) {
	var out publicCMSPage
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	endpoint := "http://" + host + "/preview/v1/cms/pages/" + url.PathEscape(slug) + "?token=" + url.QueryEscape(strings.TrimSpace(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("CMS preview %s returned %d", slug, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (a *app) fetchPublishedDesign(ctx context.Context) (publicSiteDesignEnvelope, error) {
	var out publicSiteDesignEnvelope
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/public/v1/cms/design", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("CMS design returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (a *app) fetchPreviewDesign(ctx context.Context, token string) (publicSiteDesign, error) {
	var out struct {
		Design publicSiteDesign `json:"design"`
	}
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return publicSiteDesign{}, errors.New("CMS service is not configured")
	}
	endpoint := "http://" + host + "/preview/v1/cms/design?token=" + url.QueryEscape(strings.TrimSpace(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return publicSiteDesign{}, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return publicSiteDesign{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return publicSiteDesign{}, fmt.Errorf("CMS design preview returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return publicSiteDesign{}, err
	}
	return out.Design, nil
}

func (a *app) fetchPublishedSEOSettings(ctx context.Context, locale string) (publicSEOSettings, error) {
	var out publicSEOSettings
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	endpoint := "http://"+host+"/public/v1/cms/seo?locale="+url.QueryEscape(normalizePublicLocale(locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("CMS SEO settings returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (a *app) fetchPublishedManifest(ctx context.Context, locale string) (publicCMSManifest, error) {
	var out publicCMSManifest
	host := strings.TrimSpace(a.hosts["cms"])
	if host == "" {
		return out, errors.New("CMS service is not configured")
	}
	endpoint := "http://"+host+"/public/v1/cms/manifest?locale="+url.QueryEscape(normalizePublicLocale(locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
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

type cmsMediaURL func(string) string

func cmsComponentClass(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "text"
	}
	return out
}

func dynamicCMSSectionHTML(section publicCMSSection, mediaURL cmsMediaURL) string {
	var b strings.Builder
	b.WriteString("<section class=\"himate-cms-dynamic himate-cms-")
	b.WriteString(cmsComponentClass(section.ComponentType))
	b.WriteString("\" data-cms-section=\"")
	b.WriteString(html.EscapeString(section.ID))
	b.WriteString("\"><div class=\"wrap himate-cms-dynamic-inner\">")
	if strings.TrimSpace(section.MediaAssetID) != "" {
		b.WriteString("<div class=\"himate-cms-dynamic-media\"><img loading=\"lazy\" decoding=\"async\" src=\"")
		b.WriteString(html.EscapeString(mediaURL(section.MediaAssetID)))
		b.WriteString("\" alt=\"")
		b.WriteString(html.EscapeString(section.Heading))
		b.WriteString("\"></div>")
	}
	b.WriteString("<div class=\"himate-cms-dynamic-copy\">")
	if strings.TrimSpace(section.Heading) != "" {
		b.WriteString("<h2>")
		b.WriteString(html.EscapeString(section.Heading))
		b.WriteString("</h2>")
	}
	if strings.TrimSpace(section.Body) != "" {
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(section.Body), "\n", "<br>"))
		b.WriteString("</p>")
	}
	if strings.TrimSpace(section.CTALabel) != "" && strings.TrimSpace(section.CTAURL) != "" {
		b.WriteString("<a class=\"btn btn-gold\" href=\"")
		b.WriteString(html.EscapeString(section.CTAURL))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(section.CTALabel))
		b.WriteString("</a>")
	}
	b.WriteString("</div></div></section>")
	return b.String()
}

func insertDynamicCMSSection(doc, sectionHTML string) string {
	lower := strings.ToLower(doc)
	if root := strings.Index(lower, "data-cms-dynamic-root"); root >= 0 {
		if endRel := strings.Index(lower[root:], "</main>"); endRel >= 0 {
			end := root + endRel
			return doc[:end] + sectionHTML + doc[end:]
		}
	}
	if footer := strings.Index(lower, "<footer"); footer >= 0 {
		return doc[:footer] + sectionHTML + doc[footer:]
	}
	if bodyEnd := strings.Index(lower, "</body>"); bodyEnd >= 0 {
		return doc[:bodyEnd] + sectionHTML + doc[bodyEnd:]
	}
	return doc + sectionHTML
}

func injectDynamicCMSStyles(doc string) string {
	if strings.Contains(doc, "data-himate-cms-dynamic") {
		return doc
	}
	style := "<style data-himate-cms-dynamic>" +
		".himate-cms-dynamic{padding:clamp(56px,6vw,96px) 0;background:var(--ivory,#F8F9FB);color:var(--navy,#0B1F3B)}" +
		".himate-cms-dynamic:nth-of-type(even){background:#fff}" +
		".himate-cms-dynamic-inner{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1.25fr);gap:clamp(28px,5vw,72px);align-items:center}" +
		".himate-cms-dynamic-copy h2{margin:0 0 16px;font-size:clamp(38px,4vw,64px);line-height:.96}" +
		".himate-cms-dynamic-copy p{max-width:760px;font-size:clamp(15px,1.2vw,19px);line-height:1.7;color:var(--ink,#1F2937)}" +
		".himate-cms-dynamic-media{min-height:260px;max-height:520px;overflow:hidden;border-radius:12px}" +
		".himate-cms-dynamic-media img{width:100%;height:100%;object-fit:cover}" +
		".himate-cms-hero,.himate-cms-cta{background:var(--deep,#061426);color:#fff}" +
		".himate-cms-hero .himate-cms-dynamic-copy p,.himate-cms-cta .himate-cms-dynamic-copy p{color:#e8edf3}" +
		"@media(max-width:760px){.himate-cms-dynamic-inner{grid-template-columns:1fr}.himate-cms-dynamic-media{min-height:220px}}" +
		"</style>"
	if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
		return doc[:headEnd] + style + doc[headEnd:]
	}
	return style + doc
}

func renderMarketingSection(doc string, section publicCMSSection, mediaURL cmsMediaURL) string {
	start, end, ok := marketingSectionBounds(doc, section.ID)
	if !ok {
		return insertDynamicCMSSection(doc, dynamicCMSSectionHTML(section, mediaURL))
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
		fragment = replaceFirstAttribute(fragment, "img", "src", mediaURL(section.MediaAssetID))
	}
	return doc[:start] + fragment + doc[end:]
}

func renderGlobalSEOHTML(doc string, settings publicSEOSettings) string {
	if len(settings.GlobalKeywords) > 0 {
		doc = replaceHeadTag(doc, "name=\"keywords\"", "<meta name=\"keywords\" content=\""+html.EscapeString(strings.Join(settings.GlobalKeywords, ", "))+"\">")
	}
	if mediaID := strings.TrimSpace(settings.DefaultOGImageAssetID); mediaID != "" &&
		!strings.Contains(strings.ToLower(doc), "property=\"og:image\"") {
		doc = replaceHeadTag(doc, "property=\"og:image\"", "<meta property=\"og:image\" content=\"/public/v1/cms/media/"+url.PathEscape(mediaID)+"\">")
	}
	if strings.TrimSpace(settings.OrganizationName) != "" || strings.TrimSpace(settings.OrganizationURL) != "" {
		structured := map[string]any{
			"@context": "https://schema.org",
			"@type": "Organization",
			"name": settings.OrganizationName,
			"url": settings.OrganizationURL,
		}
		if raw, err := json.Marshal(structured); err == nil {
			if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
				tag := "<script type=\"application/ld+json\" data-himate-seo=\"organization\">"+string(raw)+"</script>"
				doc = doc[:headEnd] + tag + doc[headEnd:]
			}
		}
	}
	return doc
}

func renderCMSHTML(doc string, page publicCMSPage, requestURL string, mediaURL cmsMediaURL, marker string) string {
	if normalizePublicLocale(page.Locale) == "hu_HU" {
		doc = strings.Replace(doc, "<html lang=\"en\">", "<html lang=\"hu\">", 1)
	}
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
	if page.SEO.NoIndex || marker == "PREVIEW" {
		robots = "noindex,nofollow"
	}
	doc = replaceHeadTag(doc, "name=\"robots\"", "<meta name=\"robots\" content=\""+robots+"\">")
	if len(page.SEO.Keywords) > 0 {
		doc = replaceHeadTag(doc, "name=\"keywords\"", "<meta name=\"keywords\" content=\""+html.EscapeString(strings.Join(page.SEO.Keywords, ", "))+"\">")
	}
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
		doc = replaceHeadTag(doc, "property=\"og:image\"", "<meta property=\"og:image\" content=\""+html.EscapeString(mediaURL(mediaID))+"\">")
	}
	if len(page.SEO.JSONLD) > 0 {
		if raw, err := json.Marshal(page.SEO.JSONLD); err == nil {
			if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
				tag := "<script type=\"application/ld+json\">"+string(raw)+"</script>"
				doc = doc[:headEnd] + tag + doc[headEnd:]
			}
		}
	}
	if marker != "PREVIEW" && len(page.Alternates) > 0 {
		var alternateTags strings.Builder
		if href := strings.TrimSpace(page.Alternates["en_US"]); href != "" {
			alternateTags.WriteString("<link rel=\"alternate\" hreflang=\"en-US\" href=\""+html.EscapeString(href)+"\">")
			alternateTags.WriteString("<link rel=\"alternate\" hreflang=\"x-default\" href=\""+html.EscapeString(href)+"\">")
		}
		if href := strings.TrimSpace(page.Alternates["hu_HU"]); href != "" {
			alternateTags.WriteString("<link rel=\"alternate\" hreflang=\"hu-HU\" href=\""+html.EscapeString(href)+"\">")
		}
		if alternateTags.Len() > 0 {
			if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
				doc = doc[:headEnd] + alternateTags.String() + doc[headEnd:]
			}
		}
	}
	for _, id := range page.HiddenSections {
		doc = removeMarketingSection(doc, id)
	}
	hasDynamic := false
	for _, section := range page.Sections {
		if _, _, ok := marketingSectionBounds(doc, section.ID); !ok {
			hasDynamic = true
		}
		doc = renderMarketingSection(doc, section, mediaURL)
	}
	if hasDynamic {
		doc = injectDynamicCMSStyles(doc)
	}
	if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
		doc = doc[:headEnd] + "<!-- HIMATE SSR:"+marker+" -->" + doc[headEnd:]
	}
	return doc
}

func renderPublishedCMSHTML(doc string, page publicCMSPage, requestURL string) string {
	return renderCMSHTML(doc, page, requestURL, func(id string) string {
		return "/public/v1/cms/media/"+url.PathEscape(id)
	}, "PUBLISHED")
}

func renderPreviewCMSHTML(doc string, page publicCMSPage, requestURL, slug, token string) string {
	return renderCMSHTML(doc, page, requestURL, func(id string) string {
		return "/preview/v1/cms/media/"+url.PathEscape(id)+"?slug="+url.QueryEscape(slug)+"&token="+url.QueryEscape(token)
	}, "PREVIEW")
}

var gatewayDesignColorPattern = regexp.MustCompile("^#[0-9A-Fa-f]{6}$")

func designColor(value, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if gatewayDesignColorPattern.MatchString(value) {
		return value
	}
	return fallback
}

func designFontCSS(value, fallback string) string {
	switch strings.TrimSpace(value) {
	case "Cormorant Garamond":
		return "\"Cormorant Garamond\",Georgia,serif"
	case "Inter":
		return "Inter,Arial,sans-serif"
	case "Georgia":
		return "Georgia,serif"
	case "Arial":
		return "Arial,sans-serif"
	default:
		return fallback
	}
}

func replaceNavContents(doc, className, inner string) string {
	marker := "class=\"" + className + "\""
	idx := strings.Index(doc, marker)
	if idx < 0 {
		return doc
	}
	start := strings.LastIndex(strings.ToLower(doc[:idx]), "<nav")
	if start < 0 {
		return doc
	}
	openEndRel := strings.Index(doc[start:], ">")
	if openEndRel < 0 {
		return doc
	}
	contentStart := start + openEndRel + 1
	closeRel := strings.Index(strings.ToLower(doc[contentStart:]), "</nav>")
	if closeRel < 0 {
		return doc
	}
	contentEnd := contentStart + closeRel
	return doc[:contentStart] + inner + doc[contentEnd:]
}

func designNavigationHTML(design publicSiteDesign, locale, currentPath string, footer bool) string {
	items := append([]publicNavigationItem(nil), design.Navigation...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].SortOrder < items[j].SortOrder })
	var b strings.Builder
	for _, item := range items {
		if !item.Visible || strings.TrimSpace(item.URL) == "" {
			continue
		}
		label := strings.TrimSpace(item.LabelEN)
		if normalizePublicLocale(locale) == "hu_HU" {
			label = strings.TrimSpace(item.LabelHU)
		}
		if label == "" {
			continue
		}
		active := strings.TrimRight(strings.TrimSpace(item.URL), "/") == strings.TrimRight(currentPath, "/")
		if item.URL == "/" && currentPath == "/" {
			active = true
		}
		b.WriteString("<a")
		if active && !footer {
			b.WriteString(" class=\"active\" aria-current=\"page\"")
		}
		b.WriteString(" href=\"")
		b.WriteString(html.EscapeString(item.URL))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</a>")
	}
	if !footer {
		b.WriteString("<span class=\"nav-divider\" aria-hidden=\"true\"></span><a class=\"nav-login-text\" href=\"/login\">Login</a><a class=\"login-pill\" href=\"/login\">Login</a>")
	}
	return b.String()
}

func renderSiteDesignHTML(doc string, design publicSiteDesign, locale, currentPath string, logoURL cmsMediaURL) string {
	navy := designColor(design.Navy, "#0B1F3B")
	gold := designColor(design.Gold, "#D4AF6B")
	background := designColor(design.Background, "#F8F9FB")
	textColor := designColor(design.TextColor, "#1F2937")
	headingFont := designFontCSS(design.HeadingFont, "\"Cormorant Garamond\",Georgia,serif")
	bodyFont := designFontCSS(design.BodyFont, "Inter,Arial,sans-serif")
	radius := design.ButtonRadius
	if radius < 0 || radius > 40 {
		radius = 6
	}
	style := fmt.Sprintf("<style data-himate-design>:root{--navy:%s;--deep:%s;--gold:%s;--gold2:%s;--ivory:%s;--ink:%s}body{background:%s!important;color:%s!important;font-family:%s!important}h1,h2,h3,.serif{font-family:%s!important}.btn,.login-pill{border-radius:%dpx!important}</style>",
		navy, navy, gold, gold, background, textColor, background, textColor, bodyFont, headingFont, radius)
	if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
		doc = doc[:headEnd] + style + "<!-- HIMATE DESIGN:PUBLISHED_OR_PREVIEW -->" + doc[headEnd:]
	}
	headerID := strings.TrimSpace(design.Assets["header_wordmark"])
	if headerID == "" {
		headerID = strings.TrimSpace(design.LogoMediaAssetID)
	}
	footerID := strings.TrimSpace(design.Assets["footer_wordmark"])
	if footerID == "" {
		footerID = headerID
	}
	const defaultWordmark = "/brand/himate_identity_wordmark_2026.webp"
	if headerID != "" {
		doc = strings.Replace(doc, defaultWordmark, html.EscapeString(logoURL(headerID)), 1)
	}
	if footerID != "" {
		if footer := strings.Index(strings.ToLower(doc), "<footer"); footer >= 0 {
			before, after := doc[:footer], doc[footer:]
			after = strings.Replace(after, defaultWordmark, html.EscapeString(logoURL(footerID)), 1)
			doc = before + after
		}
	}
	if faviconID := strings.TrimSpace(design.Assets["favicon"]); faviconID != "" {
		doc = strings.ReplaceAll(doc, "/brand/himate_identity_favicon_32.png", html.EscapeString(logoURL(faviconID)))
	}
	if appIconID := strings.TrimSpace(design.Assets["app_icon"]); appIconID != "" {
		if headEnd := strings.Index(strings.ToLower(doc), "</head>"); headEnd >= 0 {
			tag := "<link rel=\"apple-touch-icon\" href=\""+html.EscapeString(logoURL(appIconID))+"\">"
			doc = doc[:headEnd] + tag + doc[headEnd:]
		}
	}
	doc = replaceNavContents(doc, "links", designNavigationHTML(design, locale, currentPath, false))
	doc = replaceNavContents(doc, "footer-links", designNavigationHTML(design, locale, currentPath, true))
	return doc
}

func dynamicMarketingTemplate() string {
	return "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta name=\"description\" content=\"HIMATE System\"><meta name=\"theme-color\" content=\"#0B1F3B\"><meta name=\"robots\" content=\"index,follow\"><title>HIMATE System</title><link rel=\"stylesheet\" href=\"/himate-brand-r4.css\"><script src=\"/site.js\" defer></script><link rel=\"icon\" type=\"image/png\" sizes=\"32x32\" href=\"/brand/himate_identity_favicon_32.png\"></head><body>" +
		"<header class=\"site-nav\" style=\"background:#06172C\"><div class=\"wrap nav-inner\"><a class=\"brand-logo\" href=\"/\" aria-label=\"HIMATE home\"><img src=\"/brand/himate_identity_wordmark_2026.webp\" alt=\"HIMATE System\"></a><nav class=\"links\"><a href=\"/platform\">Platform</a><a href=\"/modules\">Modules</a><a href=\"/programs\">Programs</a><a href=\"/impact\">Impact</a><a href=\"/partners\">Partners</a><a href=\"/contact\">Contact</a><span class=\"nav-divider\" aria-hidden=\"true\"></span><a class=\"nav-login-text\" href=\"/login\">Login</a><a class=\"login-pill\" href=\"/login\">Login</a></nav><button class=\"menu\" type=\"button\" aria-label=\"Open navigation\" aria-expanded=\"false\">☰</button></div></header>" +
		"<main data-cms-dynamic-root style=\"padding-top:138px\"></main>" +
		"<footer class=\"footer\"><div class=\"wrap footer-inner\"><a class=\"brand-logo footer-logo\" href=\"/\" aria-label=\"HIMATE home\"><img src=\"/brand/himate_identity_wordmark_2026.webp\" alt=\"HIMATE System\"></a><nav class=\"footer-links\" aria-label=\"Footer navigation\"><a href=\"/platform\">Platform</a><a href=\"/modules\">Modules</a><a href=\"/programs\">Programs</a><a href=\"/impact\">Impact</a><a href=\"/partners\">Partners</a><a href=\"/contact\">Contact</a></nav><div class=\"footer-tagline\"><span>A smarter future<br>for arts &amp; culture</span><em>Culture Fuels Tomorrow.</em></div></div><div class=\"wrap footer-copy\">© 2026 HIMATE System. All rights reserved.</div></footer></body></html>"
}

func marketingTemplateFilename(slug string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "landing":
		return "landing.html", true
	case "platform":
		return "platform.html", true
	case "modules":
		return "modules.html", true
	case "programs":
		return "programs.html", true
	case "impact":
		return "impact.html", true
	case "partners":
		return "partners.html", true
	case "contact":
		return "contact.html", true
	default:
		return "", false
	}
}

func (a *app) marketingTemplate(slug string) string {
	if filename, ok := marketingTemplateFilename(slug); ok {
		if raw, err := os.ReadFile(filepath.Join(filepath.Clean(a.webDir), filename)); err == nil {
			return string(raw)
		}
	}
	return dynamicMarketingTemplate()
}

func writeHTMLResponse(w http.ResponseWriter, r *http.Request, doc string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len([]byte(doc))))
	if r.Method == http.MethodHead {
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(doc))
}

func (a *app) cmsPagePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/cms-preview/"), "/")
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if slug == "" || token == "" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	page, err := a.fetchPreviewCMS(ctx, slug, token)
	cancel()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	doc := a.marketingTemplate(slug)
	doc = renderPreviewCMSHTML(doc, page, publicOrigin(r)+r.URL.Path, slug, token)
	designCtx, designCancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
	design, designErr := a.fetchPublishedDesign(designCtx)
	designCancel()
	if designErr == nil && design.Version > 0 {
		doc = renderSiteDesignHTML(doc, design.Design, page.Locale, "/"+strings.Trim(slug, "/"), func(id string) string {
			return "/public/v1/cms/media/"+url.PathEscape(id)
		})
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("X-Himate-SSR", "preview")
	w.Header().Set("X-Himate-Preview", "cms")
	writeHTMLResponse(w, r, doc, http.StatusOK)
}

func previewActiveClass(active bool) string {
	if active {
		return "active"
	}
	return ""
}

func designPreviewFrameHTML(token, viewport string) string {
	width := "1440px"
	label := "Desktop · 1440"
	switch viewport {
	case "tablet":
		width = "834px"
		label = "Tablet · 834"
	case "mobile":
		width = "390px"
		label = "Mobile · 390"
	default:
		viewport = "desktop"
	}
	base := "/design-preview?token="+url.QueryEscape(token)
	raw := base+"&viewport="+url.QueryEscape(viewport)+"&raw=1"
	return fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta name=\"robots\" content=\"noindex,nofollow\"><title>HIMATE Design Preview</title><style>html,body{margin:0;background:#111827;color:#fff;font-family:Inter,Arial,sans-serif}.bar{position:sticky;top:0;z-index:5;display:flex;gap:10px;align-items:center;padding:10px 16px;background:#06172c;border-bottom:1px solid #d4af6b}.bar a{color:#fff;text-decoration:none;border:1px solid #667085;border-radius:6px;padding:7px 10px}.bar a.active{border-color:#d4af6b;color:#f0d39a}.label{margin-left:auto;color:#cbd5e1}.stage{padding:20px;display:flex;justify-content:center;min-height:calc(100vh - 62px)}iframe{width:%s;height:calc(100vh - 88px);border:0;background:#fff;box-shadow:0 12px 40px rgba(0,0,0,.45)}</style></head><body><div class=\"bar\"><strong>HIMATE Design Preview</strong><a href=\"%s&viewport=desktop\" class=\"%s\">Desktop</a><a href=\"%s&viewport=tablet\" class=\"%s\">Tablet</a><a href=\"%s&viewport=mobile\" class=\"%s\">Mobile</a><span class=\"label\">%s</span></div><div class=\"stage\"><iframe title=\"HIMATE website design preview\" src=\"%s\"></iframe></div></body></html>",
		width,
		html.EscapeString(base), previewActiveClass(viewport=="desktop"),
		html.EscapeString(base), previewActiveClass(viewport=="tablet"),
		html.EscapeString(base), previewActiveClass(viewport=="mobile"),
		html.EscapeString(label), html.EscapeString(raw))
}

func (a *app) designPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	preview, err := a.fetchPreviewDesign(ctx, token)
	cancel()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	viewport := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("viewport")))
	if viewport != "tablet" && viewport != "mobile" {
		viewport = "desktop"
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("X-Himate-Preview", "design")
	if r.URL.Query().Get("raw") != "1" {
		writeHTMLResponse(w, r, designPreviewFrameHTML(token, viewport), http.StatusOK)
		return
	}
	doc := a.marketingTemplate("landing")
	locale := publicLocale(r)
	pageCtx, pageCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	if page, pageErr := a.fetchPublishedCMS(pageCtx, "landing", locale); pageErr == nil {
		doc = renderPublishedCMSHTML(doc, page, publicOrigin(r)+"/")
	}
	pageCancel()
	doc = renderSiteDesignHTML(doc, preview, locale, "/", func(id string) string {
		return "/preview/v1/cms/design/media/"+url.PathEscape(id)+"?token="+url.QueryEscape(token)
	})
	writeHTMLResponse(w, r, doc, http.StatusOK)
}

func (a *app) serveDynamicCMSPage(w http.ResponseWriter, r *http.Request, slug string) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if !regexp.MustCompile("^[a-z0-9]+(?:-[a-z0-9]+)*$").MatchString(slug) {
		return false
	}
	locale := publicLocale(r)
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	page, err := a.fetchPublishedCMS(ctx, slug, locale)
	cancel()
	if err != nil {
		return false
	}
	doc := renderPublishedCMSHTML(dynamicMarketingTemplate(), page, publicOrigin(r)+r.URL.Path)
	designCtx, designCancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
	design, designErr := a.fetchPublishedDesign(designCtx)
	designCancel()
	if designErr == nil && design.Version > 0 {
		doc = renderSiteDesignHTML(doc, design.Design, locale, r.URL.Path, func(id string) string {
			return "/public/v1/cms/media/"+url.PathEscape(id)
		})
		w.Header().Set("X-Himate-Design", "published")
	}
	w.Header().Set("X-Himate-SSR", "published")
	w.Header().Set("X-Himate-SEO", "page+global")
	w.Header().Set("Content-Language", map[bool]string{true:"hu",false:"en"}[locale=="hu_HU"])
	writeHTMLResponse(w, r, doc, http.StatusOK)
	return true
}

func (a *app) serveMarketingPage(w http.ResponseWriter, r *http.Request, filename, slug string) {
	path := filepath.Join(filepath.Clean(a.webDir), filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	doc := string(raw)
	locale := publicLocale(r)
	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Millisecond)
	page, cmsErr := a.fetchPublishedCMS(ctx, slug, locale)
	cancel()
	if cmsErr == nil {
		doc = renderPublishedCMSHTML(doc, page, publicOrigin(r)+r.URL.Path)
		w.Header().Set("X-Himate-SSR", "published")
		w.Header().Set("X-Himate-SEO", "page+global")
		w.Header().Set("Content-Language", map[bool]string{true:"hu",false:"en"}[locale=="hu_HU"])
	} else {
		seoCtx, seoCancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
		settings, seoErr := a.fetchPublishedSEOSettings(seoCtx, locale)
		seoCancel()
		if seoErr == nil {
			doc = renderGlobalSEOHTML(doc, settings)
			w.Header().Set("X-Himate-SEO", "global")
		}
		w.Header().Set("X-Himate-SSR", "static-fallback")
	}
	designCtx, designCancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
	design, designErr := a.fetchPublishedDesign(designCtx)
	designCancel()
	if designErr == nil && design.Version > 0 {
		doc = renderSiteDesignHTML(doc, design.Design, locale, r.URL.Path, func(id string) string {
			return "/public/v1/cms/media/"+url.PathEscape(id)
		})
		w.Header().Set("X-Himate-Design", "published")
	}
	writeHTMLResponse(w, r, doc, http.StatusOK)
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
	manifest, err := a.fetchPublishedManifest(ctx, publicLocale(r))
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

		if r.URL.Path == "/login" || r.URL.Path == "/app" || strings.HasPrefix(r.URL.Path, "/app/") ||
			r.URL.Path == "/partner/login" || r.URL.Path == "/partner/app" || strings.HasPrefix(r.URL.Path, "/partner/app/") {
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

		slug := strings.Trim(strings.TrimSpace(r.URL.Path), "/")
		if slug != "" && !strings.Contains(slug, "/") && a.serveDynamicCMSPage(w, r, slug) {
			return
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
		designPreviewFrame := r.URL.Path == "/design-preview"
		if designPreviewFrame {
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		} else {
			w.Header().Set("X-Frame-Options", "DENY")
		}
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
		if designPreviewFrame {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; form-action 'self'; frame-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; connect-src 'self' https://fonts.gstatic.com; font-src 'self' data: https://fonts.gstatic.com; frame-ancestors 'self'")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; form-action 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; connect-src 'self' https://fonts.gstatic.com; font-src 'self' data: https://fonts.gstatic.com; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}
