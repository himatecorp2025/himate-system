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
