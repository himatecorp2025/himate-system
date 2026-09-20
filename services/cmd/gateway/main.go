package main

import (
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
	secureCookie     bool
	client           *http.Client
	proxies          map[string]*httputil.ReverseProxy
	hosts            map[string]string
	dashboardMu      sync.RWMutex
	dashboardPayload map[string]any
	dashboardExpires time.Time
	loginMu          sync.Mutex
	loginAttempts    map[string]loginState
}

type loginState struct {
	Failures     int
	WindowStart  time.Time
	BlockedUntil time.Time
}

type user struct {
	ID, Name, Email, PasswordHash string
	Roles                         []string
	Active                        bool
}
type claims struct {
	Sub, Email, Name string
	Roles            []string
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
	secure, _ := strconv.ParseBool(common.Env("COOKIE_SECURE", "true"))
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	}
	a := &app{
		db: db, secret: os.Getenv("HIMATE_SESSION_SECRET"), internalToken: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		webDir: common.Env("WEB_DIST_DIR", "/app/web"), env: common.Env("HIMATE_ENV", "development"),
		version: common.Env("HIMATE_APP_VERSION", "0.2.0-start-04-08"), ttl: time.Duration(ttlHours) * time.Hour,
		secureCookie: secure, client: &http.Client{Timeout: 4 * time.Second, Transport: transport},
		proxies: map[string]*httputil.ReverseProxy{},
		loginAttempts: map[string]loginState{},
		hosts: map[string]string{
			"partners": os.Getenv("PARTNERS_HOSTPORT"),
			"catalog":  os.Getenv("CATALOG_HOSTPORT"),
			"billing":  os.Getenv("BILLING_HOSTPORT"),
			"contact":  os.Getenv("CONTACT_HOSTPORT"),
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
	for name, host := range a.hosts {
		p, err := newProxy(host, a.internalToken)
		if err != nil {
			log.Error("proxy", "service", name, "error", err)
			os.Exit(1)
		}
		a.proxies[name] = p
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", a.health)
	mux.HandleFunc("/api/v1/auth/login", a.login)
	mux.HandleFunc("/api/v1/auth/logout", a.logout)
	mux.HandleFunc("/api/v1/auth/me", a.me)
	mux.HandleFunc("/api/v1/public/contact", a.publicContact)
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
	_, err = a.db.ExecContext(ctx, `INSERT INTO identity.users(id,name,email,password_hash,roles,active)
		VALUES('usr_bootstrap_001',$1,$2,$3,$4::jsonb,TRUE)
		ON CONFLICT(email) DO UPDATE SET name=EXCLUDED.name,password_hash=EXCLUDED.password_hash,roles=EXCLUDED.roles,active=TRUE`,
		name, email, hashed, string(roles))
	return err
}

func clientKey(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if i := strings.IndexByte(forwarded, ','); i >= 0 { forwarded = forwarded[:i] }
		return strings.TrimSpace(forwarded)
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
	var in struct{ Email, Password string }
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
	token, _ := a.issueSession(u)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: a.secureCookie, SameSite: http.SameSiteStrictMode,
		MaxAge: int(a.ttl.Seconds()),
	})
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

func hasRole(u user, role string) bool {
	for _, value := range u.Roles {
		if value == role { return true }
	}
	return false
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
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !hasRole(u, "platform_admin") {
		common.APIError(w, 403, "FORBIDDEN", "Platform administrator permission is required")
		return
	}
	r.Header.Set("X-Himate-User-ID", u.ID)
	if r.URL.Path == "/api/v1/dashboard/summary" {
		a.dashboard(w, r)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/partners" && r.Method == http.MethodGet:
		a.partnerPortfolio(w, r)
	case r.URL.Path == "/api/v1/partners", r.URL.Path == "/api/v1/partner-categories":
		a.proxies["partners"].ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/") && strings.Contains(r.URL.Path, "/modules"):
		a.proxies["catalog"].ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/partners/"):
		a.proxies["partners"].ServeHTTP(w, r)
	case r.URL.Path == "/api/v1/modules", r.URL.Path == "/api/v1/module-groups", strings.HasPrefix(r.URL.Path, "/api/v1/modules/"):
		a.proxies["catalog"].ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/billing/"):
		a.proxies["billing"].ServeHTTP(w, r)
	default:
		common.APIError(w, 404, "API_NOT_FOUND", "API endpoint not found")
	}
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{"identity": "ok"}
	overall := "ok"
	checkedAt := time.Now().UTC()
	for name, host := range a.hosts {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/health", nil)
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
	common.JSON(w, 200, map[string]any{"status": overall, "service": "himate-gateway", "environment": a.env, "version": a.version, "architecture": "containerized-microservices", "checked_at": checkedAt, "services": services})
}

func (a *app) partnerPortfolio(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	type partnerPage struct {
		Items   []map[string]any `json:"items"`
		Count   int              `json:"count"`
		Total   int              `json:"total"`
		Limit   int              `json:"limit"`
		Offset  int              `json:"offset"`
		HasMore bool             `json:"has_more"`
	}
	type portfolioPage struct { Items []map[string]any `json:"items"` }

	var partners partnerPage
	var catalogPortfolio, billingPortfolio portfolioPage
	var partnerErr, catalogErr, billingErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		path := "/api/v1/partners"
		if r.URL.RawQuery != "" { path += "?" + r.URL.RawQuery }
		partnerErr = a.internalGET(ctx, a.hosts["partners"], path, &partners)
	}()
	go func() {
		defer wg.Done()
		catalogErr = a.internalGET(ctx, a.hosts["catalog"], "/internal/v1/portfolio", &catalogPortfolio)
	}()
	go func() {
		defer wg.Done()
		billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/portfolio", &billingPortfolio)
	}()
	wg.Wait()

	if partnerErr != nil {
		common.APIError(w, 502, "PARTNERS_UNAVAILABLE", "Partner portfolio is temporarily unavailable")
		return
	}

	catalogByID := map[string]map[string]any{}
	for _, item := range catalogPortfolio.Items { catalogByID[fmt.Sprint(item["partner_id"])] = item }
	billingByID := map[string]map[string]any{}
	for _, item := range billingPortfolio.Items { billingByID[fmt.Sprint(item["partner_id"])] = item }

	for _, p := range partners.Items {
		id := fmt.Sprint(p["id"])
		cat := catalogByID[id]
		bill := billingByID[id]
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
	}
	if catalogErr != nil || billingErr != nil {
		w.Header().Set("X-Himate-Portfolio", "partial")
	}
	w.Header().Set("Server-Timing", fmt.Sprintf("partner-portfolio;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, 200, map[string]any{
		"items": partners.Items, "count": partners.Count, "total": partners.Total,
		"limit": partners.Limit, "offset": partners.Offset, "has_more": partners.HasMore,
	})
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

	var partnerResponse, moduleResponse struct {
		Items []map[string]any `json:"items"`
	}
	var partnerErr, moduleErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		partnerErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partners", &partnerResponse)
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

	live := 0
	for _, p := range partnerResponse.Items {
		if p["lifecycle"] == "LIVE" {
			live++
		}
	}
	status := "healthy"
	if partnerErr != nil || moduleErr != nil {
		status = "degraded"
	}
	payload := map[string]any{
		"partners": map[string]any{"total": len(partnerResponse.Items), "live": live},
		"modules": map[string]any{"catalog_total": len(moduleResponse.Items)},
		"system": map[string]any{
			"status": status, "environment": a.env, "version": a.version, "architecture": "microservices",
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+path, nil)
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

func (a *app) findUser(field, value string) (user, error) {
	if field != "email" && field != "id" {
		return user{}, errors.New("invalid lookup")
	}
	var u user
	var raw []byte
	err := a.db.QueryRow(`SELECT id,name,email,password_hash,roles,active FROM identity.users WHERE `+field+`=$1`, value).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &raw, &u.Active)
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
	return u, nil
}
func publicUser(u user) map[string]any {
	return map[string]any{"id": u.ID, "name": u.Name, "email": u.Email, "roles": u.Roles}
}

func (a *app) issueSession(u user) (string, error) {
	raw, _ := json.Marshal(claims{Sub: u.ID, Email: u.Email, Name: u.Name, Roles: u.Roles, Exp: time.Now().Add(a.ttl).Unix()})
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

func (a *app) web() http.Handler {
	root := filepath.Clean(a.webDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			landing := filepath.Join(root, "landing.html")
			if _, err := os.Stat(landing); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeFile(w, r, landing)
				return
			}
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
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeFile(w, r, filepath.Join(root, page))
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
		} else if strings.HasPrefix(path, "/art/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "/" || path == "/login" || path == "/app" || path == "/platform" || path == "/modules" || path == "/programs" || path == "/impact" || path == "/partners" || path == "/contact" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "no-store, max-age=0, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".wasm") {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; connect-src 'self' https://fonts.gstatic.com; font-src 'self' data: https://fonts.gstatic.com; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
