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


func TestPasswordComplexityPolicy(t *testing.T) {
	valid := []string{"Strong-Password1!", "Longer_Passphrase9#"}
	for _, value := range valid {
		if message := passwordPolicyError(value); message != "" {
			t.Fatalf("expected %q to satisfy policy: %s", value, message)
		}
	}
	invalid := []string{"Short1!", "alllowercase1!", "ALLUPPERCASE1!", "NoNumberHere!", "NoSpecial1234"}
	for _, value := range invalid {
		if message := passwordPolicyError(value); message == "" {
			t.Fatalf("expected %q to be rejected", value)
		}
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
		{"marketing_admin", "cms.read", true},
		{"marketing_admin", "cms.write", true},
		{"marketing_admin", "cms.approve", true},
		{"marketing_admin", "contact.read", true},
		{"marketing_admin", "contact.write", true},
		{"marketing_admin", "billing.read", false},
		{"marketing_admin", "administration.read", false},
	}
	a := &app{}
	for _, tc := range tests {
		u := user{Roles: []string{tc.role}}
		if got := a.hasPermission(u, tc.permission); got != tc.want {
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
		{http.MethodGet, "/api/v1/notifications", "notifications.read"},
		{http.MethodPost, "/api/v1/notifications/read-all", "notifications.write"},
		{http.MethodPost, "/api/v1/admin/roles", "administration.approve"},
		{http.MethodGet, "/api/v1/partners", "partners.read"},
		{http.MethodPatch, "/api/v1/partners/ptr_1", "partners.write"},
		{http.MethodGet, "/api/v1/partners/ptr_1/modules", "catalog.read"},
		{http.MethodPatch, "/api/v1/partners/ptr_1/modules/mod_1", "catalog.write"},
		{http.MethodPut, "/api/v1/billing/partners/ptr_1/terms", "billing.write"},
		{http.MethodPut, "/api/v1/billing/partners/ptr_1/license", "billing.approve"},
		{http.MethodPost, "/api/v1/provisioning/jobs/job_1/run", "provisioning.approve"},
		{http.MethodPost, "/api/v1/environments/env_production_1/verify-domain", "environments.write"},
		{http.MethodPost, "/api/v1/environments/env_production_1/deploy", "environments.approve"},
		{http.MethodPost, "/api/v1/environments/env_production_1/launch", "environments.approve"},
		{http.MethodPatch, "/api/v1/evidence/ev_1", "evidence.approve"},
		{http.MethodPost, "/api/v1/cms/pages/page_1/publish", "cms.approve"},
		{http.MethodPost, "/api/v1/cms/pages/page_1/rollback", "cms.approve"},
		{http.MethodPut, "/api/v1/cms/design/draft", "cms.write"},
		{http.MethodPost, "/api/v1/cms/design/publish", "cms.approve"},
		{http.MethodGet, "/api/v1/contact/inquiries", "contact.read"},
		{http.MethodPatch, "/api/v1/contact/inquiries/inq_1", "contact.write"},
		{http.MethodGet, "/api/v1/cms/seo/audit", "cms.read"},
		{http.MethodPut, "/api/v1/cms/seo/draft", "cms.write"},
		{http.MethodPost, "/api/v1/cms/seo/publish", "cms.approve"},
		{http.MethodGet, "/api/v1/admin/users", "administration.read"},
		{http.MethodPost, "/api/v1/admin/users", "administration.approve"},
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
		{"/api/v1/cms/design/publish", "cms", ""},
		{"/api/v1/contact/inquiries/inq_1", "contact", ""},
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

func TestBrandAssetServesThroughStaticWebRoot(t *testing.T) {
	root := t.TempDir()
	brandDir := filepath.Join(root, "brand")
	if err := os.MkdirAll(brandDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("RIFF-current-HIMATE-WEBP")
	if err := os.WriteFile(filepath.Join(brandDir, "himate_identity_wordmark_2026.webp"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app{webDir: root}
	req := httptest.NewRequest(http.MethodGet, "/brand/himate_identity_wordmark_2026.webp", nil)
	rec := httptest.NewRecorder()
	a.web().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected brand asset 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("expected image/webp, got %q", got)
	}
	if rec.Body.String() != string(body) {
		t.Fatalf("unexpected brand asset body %q", rec.Body.String())
	}
}

func TestUnknownBrandAssetReturns404(t *testing.T) {
	a := &app{webDir: t.TempDir()}
	req := httptest.NewRequest(http.MethodGet, "/brand/unknown-identity.webp", nil)
	rec := httptest.NewRecorder()
	a.web().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
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


func TestRenderPublishedCMSHTML(t *testing.T) {
	template := `<!doctype html><html><head><meta name="description" content="fallback"><title>Fallback</title></head><body><section data-cms-section="hero"><h1>Fallback heading</h1><p>Fallback body</p><a href="/old">Old CTA</a><img src="/old.webp"></section><section data-cms-section="hidden"><h2>Hidden static content</h2></section></body></html>`
	page := publicCMSPage{
		Slug: "platform",
		SEO: publicCMSSEO{
			Title: "Server Rendered HIMATE",
			MetaDescription: "SSR description",
			Canonical: "https://www.himate.com/platform",
			OGTitle: "SSR OG",
			OGDescription: "SSR OG description",
			OGImageAssetID: "media_1",
		},
		Sections: []publicCMSSection{{
			ID: "hero", ComponentType: "HERO", Heading: "Published heading", Body: "Published body",
			CTALabel: "Published CTA", CTAURL: "/contact", MediaAssetID: "media_1", Visible: true,
		}},
		HiddenSections: []string{"hidden"},
	}
	got := renderPublishedCMSHTML(template, page, "https://fallback.invalid/platform")
	for _, want := range []string{
		"<title>Server Rendered HIMATE</title>",
		`name="description" content="SSR description"`,
		`rel="canonical" href="https://www.himate.com/platform"`,
		`property="og:title" content="SSR OG"`,
		`property="og:description" content="SSR OG description"`,
		`property="og:image" content="/public/v1/cms/media/media_1"`,
		"Published heading", "Published body", "Published CTA", `href="/contact"`,
		`src="/public/v1/cms/media/media_1"`, "HIMATE SSR:PUBLISHED",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected rendered HTML to contain %q: %s", want, got)
		}
	}
	if strings.Contains(got, "Hidden static content") {
		t.Fatalf("hidden CMS section remained in server-rendered HTML: %s", got)
	}
}


func TestAuditSanitization(t *testing.T) {
	input := map[string]any{
		"name": "safe",
		"password": "super-secret",
		"nested": map[string]any{
			"api_token": "token-value",
			"value": "visible",
		},
	}
	got := sanitizeAuditValue(input).(map[string]any)
	if got["password"] != "[REDACTED]" {
		t.Fatalf("password was not redacted: %#v", got)
	}
	nested := got["nested"].(map[string]any)
	if nested["api_token"] != "[REDACTED]" || nested["value"] != "visible" {
		t.Fatalf("nested audit sanitization failed: %#v", nested)
	}
}

func TestAuditActionClassification(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/api/v1/admin/users", "ADMIN_USER_CREATED"},
		{http.MethodPatch, "/api/v1/admin/users/usr_1", "ADMIN_USER_UPDATED"},
		{http.MethodPost, "/api/v1/cms/pages/page_1/publish", "CMS_PAGE_PUBLISHED"},
		{http.MethodPost, "/api/v1/environments/env_1/deploy", "ENVIRONMENT_DEPLOY"},
		{http.MethodPost, "/api/v1/environments/env_1/launch", "ENVIRONMENT_LAUNCH"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "https://himate.example"+tc.path, nil)
		if got := auditAction(req); got != tc.want {
			t.Fatalf("%s %s: expected %s, got %s", tc.method, tc.path, tc.want, got)
		}
	}
}

func TestSecurityHeadersAddsCorrelationID(t *testing.T) {
	var gotRequestID, gotCorrelationID string
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get("X-Request-ID")
		gotCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "https://himate.example/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if gotRequestID == "" || gotCorrelationID == "" {
		t.Fatalf("request tracing headers missing: request=%q correlation=%q", gotRequestID, gotCorrelationID)
	}
	if rec.Header().Get("X-Correlation-ID") != gotCorrelationID {
		t.Fatalf("correlation ID was not returned to the client")
	}
}


func TestProfileLocaleNormalization(t *testing.T) {
	if got := normalizedLocale("hu-HU"); got != "hu_HU" {
		t.Fatalf("expected hu_HU, got %q", got)
	}
	if got := normalizedLocale("unknown"); got != "en_US" {
		t.Fatalf("unknown locale must fall back to en_US, got %q", got)
	}
	if got := normalizedTimezone("Europe/Budapest"); got != "Europe/Budapest" {
		t.Fatalf("valid timezone changed: %q", got)
	}
	if got := normalizedTimezone("../bad zone"); got != "UTC" {
		t.Fatalf("unsafe timezone must fall back to UTC, got %q", got)
	}
}

func TestProfileAuditActions(t *testing.T) {
	cases := []struct {
		method string
		path string
		want string
	}{
		{http.MethodPatch, "/api/v1/profile", "PROFILE_UPDATED"},
		{http.MethodPost, "/api/v1/profile/password", "PROFILE_PASSWORD_CHANGED"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "https://himate.example"+tc.path, nil)
		if got := auditAction(req); got != tc.want {
			t.Fatalf("%s %s: expected %s, got %s", tc.method, tc.path, tc.want, got)
		}
	}
}

func TestPublicLocaleNormalization(t *testing.T) {
	tests := map[string]string{
		"": "en_US",
		"en": "en_US",
		"en-US": "en_US",
		"hu": "hu_HU",
		"hu_HU": "hu_HU",
		"hu-HU": "hu_HU",
	}
	for value, want := range tests {
		if got := normalizePublicLocale(value); got != want {
			t.Fatalf("%q => %q, want %q", value, got, want)
		}
	}
}

func TestPublishedSEOHeadRendering(t *testing.T) {
	doc := "<html lang=\"en\"><head><title>Static title</title><meta name=\"description\" content=\"static\"><meta name=\"robots\" content=\"index,follow\"></head><body></body></html>"
	page := publicCMSPage{
		Locale: "hu_HU",
		SEO: publicCMSSEO{
			Title: "HIMATE magyar SEO oldal",
			MetaDescription: "Magyar SEO leírás a szerveroldali renderelés teszteléséhez.",
			Canonical: "https://www.himate.com/seo-test-hu",
			Keywords: []string{"kultúra", "művészet", "HIMATE"},
			JSONLD: map[string]any{"@context":"https://schema.org","@type":"WebPage","inLanguage":"hu-HU"},
		},
		Alternates: map[string]string{
			"en_US": "https://www.himate.com/seo-test",
			"hu_HU": "https://www.himate.com/seo-test-hu",
		},
	}
	rendered := renderPublishedCMSHTML(doc, page, "https://www.himate.com/seo-test-hu")
	for _, required := range []string{
		"<html lang=\"hu\">",
		"name=\"keywords\" content=\"kultúra, művészet, HIMATE\"",
		"application/ld+json",
		"hreflang=\"en-US\"",
		"hreflang=\"hu-HU\"",
		"hreflang=\"x-default\"",
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered SEO head missing %q: %s", required, rendered)
		}
	}
}

func TestGlobalSEOFallbackRendering(t *testing.T) {
	doc := "<html lang=\"en\"><head><title>Static Modules</title><meta name=\"description\" content=\"Static description\"></head><body></body></html>"
	settings := publicSEOSettings{
		Locale:                "hu_HU",
		Version:               3,
		GlobalKeywords:        []string{"művészet", "kultúra", "HIMATE"},
		OrganizationName:      "HIMATE System",
		OrganizationURL:       "https://www.himate.com",
		DefaultOGImageAssetID: "cms_media_123",
	}
	rendered := renderGlobalSEOHTML(doc, settings)
	for _, required := range []string{
		"name=\"keywords\" content=\"művészet, kultúra, HIMATE\"",
		"property=\"og:image\" content=\"/public/v1/cms/media/cms_media_123\"",
		"application/ld+json",
		"data-himate-seo=\"organization\"",
		"\"@type\":\"Organization\"",
		"\"name\":\"HIMATE System\"",
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("global SEO fallback output missing %q: %s", required, rendered)
		}
	}
}
