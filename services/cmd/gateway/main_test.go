package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := hashPassword("a-strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(h, "a-strong-password-123") {
		t.Fatal("expected password to verify")
	}
	if verifyPassword(h, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}

func TestRequestOriginProtection(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://himate.example/api/v1/partners", nil)
	req.Host = "himate.example"
	req.Header.Set("Origin", "https://evil.example")
	if requestOriginAllowed(req) {
		t.Fatal("cross-site origin must be rejected")
	}

	same := httptest.NewRequest(http.MethodPost, "https://himate.example/api/v1/partners", nil)
	same.Host = "himate.example"
	same.Header.Set("Origin", "https://himate.example")
	if !requestOriginAllowed(same) {
		t.Fatal("same-origin request should be allowed")
	}
}

func TestLoginAttemptThrottling(t *testing.T) {
	a := &app{loginAttempts: map[string]loginState{}}
	now := time.Now().UTC()
	key := "203.0.113.10"
	for i := 0; i < 5; i++ {
		a.recordLoginFailure(key, now.Add(time.Duration(i)*time.Second))
	}
	if a.loginAllowed(key, now.Add(10*time.Second)) {
		t.Fatal("expected client to be blocked after five failures")
	}
	if !a.loginAllowed(key, now.Add(16*time.Minute)) {
		t.Fatal("expected client block to expire")
	}
}

func TestMarketingFrontendServesFreshAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "landing.html"), []byte("<html>brand</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>app</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "himate-brand-r4.css"), []byte("body{color:#0B1F3B}"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app{webDir: root}
	handler := securityHeaders(a.web())

	for _, path := range []string{"/", "/himate-brand-r4.css", "/app/partners/ptr_000001/modules"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
			t.Fatalf("%s: expected no-store cache policy, got %q", path, got)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/technology", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("legacy route: expected 308, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/platform" {
		t.Fatalf("legacy route: expected /platform, got %q", got)
	}
}


func TestSTART19RolePermissionMatrix(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		want       bool
	}{
		{"platform_admin", "billing.approve", true},
		{"platform_admin", "provisioning.approve", true},
		{"operations_admin", "partners.write", true},
		{"operations_admin", "provisioning.approve", true},
		{"operations_admin", "billing.write", false},
		{"finance_admin", "billing.approve", true},
		{"finance_admin", "catalog.read", true},
		{"finance_admin", "environments.write", false},
		{"reporting_admin", "impact.write", true},
		{"reporting_admin", "evidence.approve", true},
		{"reporting_admin", "billing.read", false},
		{"reporting_admin", "administration.read", false},
	}
	for _, tc := range tests {
		u := user{Roles: []string{tc.role}}
		if got := hasPermission(u, tc.permission); got != tc.want {
			t.Fatalf("%s permission %s = %v, want %v", tc.role, tc.permission, got, tc.want)
		}
	}
}

func TestSTART19RequiredPermissionClassification(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/v1/dashboard/summary", "dashboard.read"},
		{http.MethodGet, "/api/v1/partners", "partners.read"},
		{http.MethodPatch, "/api/v1/partners/ptr_1", "partners.write"},
		{http.MethodGet, "/api/v1/partners/ptr_1/modules", "catalog.read"},
		{http.MethodPatch, "/api/v1/partners/ptr_1/modules/mod_1", "catalog.write"},
		{http.MethodPut, "/api/v1/billing/partners/ptr_1/terms", "billing.write"},
		{http.MethodPut, "/api/v1/billing/partners/ptr_1/license", "billing.approve"},
		{http.MethodPost, "/api/v1/provisioning/jobs/job_1/run", "provisioning.approve"},
		{http.MethodPatch, "/api/v1/evidence/ev_1", "evidence.approve"},
		{http.MethodPost, "/api/v1/cms/pages/page_1/publish", "cms.approve"},
		{http.MethodPost, "/api/v1/cms/pages/page_1/rollback", "cms.approve"},
		{http.MethodGet, "/api/v1/admin/users", "administration.read"},
		{http.MethodPost, "/api/v1/admin/users", "administration.write"},
		{http.MethodPatch, "/api/v1/admin/users/usr_1", "administration.approve"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := requiredPermission(req); got != tc.want {
			t.Fatalf("%s %s => %s, want %s", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestSTART19NormalizeRoles(t *testing.T) {
	roles, err := normalizeRoles([]string{"finance_admin", "finance_admin", "reporting_admin"})
	if err != nil {
		t.Fatalf("unexpected normalize error: %v", err)
	}
	if len(roles) != 2 || roles[0] != "finance_admin" || roles[1] != "reporting_admin" {
		t.Fatalf("unexpected normalized roles: %#v", roles)
	}
	if _, err := normalizeRoles([]string{"root"}); err == nil {
		t.Fatal("unknown role must be rejected")
	}
	if _, err := normalizeRoles(nil); err == nil {
		t.Fatal("empty roles must be rejected")
	}
}

func TestAuditResourceClassification(t *testing.T) {
	tests := []struct {
		path string
		wantResource string
		wantPartner string
	}{
		{"/api/v1/partners/ptr_123", "partners", "ptr_123"},
		{"/api/v1/billing/partners/ptr_456/terms", "billing", "ptr_456"},
		{"/api/v1/connectors/ptr_789/credential", "connectors", "ptr_789"},
		{"/api/v1/cms/pages/page_1/publish", "cms", ""},
		{"/api/v1/impact/values?partner_id=ptr_900", "impact", "ptr_900"},
		{"/api/v1/modules/demo", "catalog", ""},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		resource, partnerID := auditResource(req)
		if resource != tc.wantResource || partnerID != tc.wantPartner {
			t.Fatalf("%s => (%s,%s), want (%s,%s)", tc.path, resource, partnerID, tc.wantResource, tc.wantPartner)
		}
	}
}

func TestAuditResponseWriterCapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &auditResponseWriter{ResponseWriter: rec}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("ok"))
	if w.status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.status)
	}
}

func TestRememberSessionUsesRequestedTTL(t *testing.T) {
	a := &app{secret: "test-secret", ttl: 8 * time.Hour}
	u := user{ID: "usr_test", Email: "test@example.com", Name: "Test User", Roles: []string{"platform_admin"}, Active: true}

	shortToken, err := a.issueSession(u, 8*time.Hour)
	if err != nil { t.Fatal(err) }
	longToken, err := a.issueSession(u, 30*24*time.Hour)
	if err != nil { t.Fatal(err) }

	shortClaims, err := a.parseSession(shortToken)
	if err != nil { t.Fatal(err) }
	longClaims, err := a.parseSession(longToken)
	if err != nil { t.Fatal(err) }

	if longClaims.Exp-shortClaims.Exp < int64((29*24*time.Hour)/time.Second) {
		t.Fatalf("remember session was not materially longer: short=%d long=%d", shortClaims.Exp, longClaims.Exp)
	}
}

func TestBrandLogoFallsBackToFlutterBundle(t *testing.T) {
	root := t.TempDir()
	assetDir := filepath.Join(root, "assets", "assets")
	if err := os.MkdirAll(assetDir, 0o700); err != nil { t.Fatal(err) }
	body := []byte("RIFF-fallback-WEBP")
	if err := os.WriteFile(filepath.Join(assetDir, "himate_logo_master_v2.webp"), body, 0o600); err != nil { t.Fatal(err) }

	a := &app{webDir: root}
	req := httptest.NewRequest(http.MethodGet, "/art/himate_logo_master_v2.webp", nil)
	rec := httptest.NewRecorder()
	a.brandLogo(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("expected logo 200, got %d", rec.Code) }
	if rec.Body.String() != string(body) { t.Fatalf("unexpected fallback body %q", rec.Body.String()) }
	if got := rec.Header().Get("X-Himate-Logo-Source"); !strings.Contains(got, "assets") {
		t.Fatalf("expected Flutter bundle fallback source, got %q", got)
	}
}

func TestBrandLogoRouteServesVersionedAsset(t *testing.T) {
	root := t.TempDir()
	art := filepath.Join(root, "art")
	if err := os.MkdirAll(art, 0o700); err != nil { t.Fatal(err) }
	body := []byte("RIFF-test-webp")
	if err := os.WriteFile(filepath.Join(art, "himate_logo_master_v2.webp"), body, 0o600); err != nil { t.Fatal(err) }

	a := &app{webDir: root}
	req := httptest.NewRequest(http.MethodGet, "/art/himate_logo_master_v2.webp", nil)
	rec := httptest.NewRecorder()
	a.brandLogo(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("expected logo 200, got %d", rec.Code) }
	if got := rec.Header().Get("Content-Type"); got != "image/webp" { t.Fatalf("expected image/webp, got %q", got) }
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") { t.Fatalf("expected no-store, got %q", got) }
	if rec.Body.String() != string(body) { t.Fatalf("unexpected logo body %q", rec.Body.String()) }
}

func TestGatewayLivenessDoesNotDependOnPrivateServices(t *testing.T) {
	a := &app{env: "production", version: "test"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	rec := httptest.NewRecorder()

	a.live(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected liveness 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected liveness ok payload, got %s", rec.Body.String())
	}
}

func TestGatewayHealthReportsMissingBindingAsDegraded(t *testing.T) {
	a := &app{
		env: "production",
		version: "test",
		client: &http.Client{Timeout: time.Second},
		hosts: map[string]string{"health": ""},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	a.health(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected health endpoint 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"degraded"`) {
		t.Fatalf("expected degraded status, got %s", body)
	}
	if !strings.Contains(body, `"health":"unconfigured"`) {
		t.Fatalf("expected unconfigured health binding, got %s", body)
	}
}

func TestMissingProxyReturnsServiceUnavailable(t *testing.T) {
	a := &app{proxies: map[string]*httputil.ReverseProxy{}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system-health", nil)
	rec := httptest.NewRecorder()

	a.serveProxy(rec, req, "health")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing proxy, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("expected structured service unavailable error, got %s", rec.Body.String())
	}
}
