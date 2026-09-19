package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestMarketingFrontendServesFreshAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "landing.html"), []byte("<html>brand</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "himate-brand-r4.css"), []byte("body{color:#0B1F3B}"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app{webDir: root}
	handler := securityHeaders(a.web())

	for _, path := range []string{"/", "/himate-brand-r4.css"} {
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
