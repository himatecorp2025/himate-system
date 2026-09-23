package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseGuardPublishesVersionHeaders(t *testing.T) {
	t.Setenv("HIMATE_APP_VERSION", "0.8.26-start-23.11.3i")
	handler := ReleaseGuard("partners", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Himate-App-Version"); got != "0.8.26-start-23.11.3i" {
		t.Fatalf("unexpected version header %q", got)
	}
	if got := rec.Header().Get("X-Himate-Service"); got != "partners" {
		t.Fatalf("unexpected service header %q", got)
	}
}

func TestReleaseGuardBlocksMismatchedInternalMutation(t *testing.T) {
	t.Setenv("HIMATE_APP_VERSION", "0.8.26-start-23.11.3i")
	called := false
	handler := ReleaseGuard("partners", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		JSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/partners", strings.NewReader("{}"))
	req.Header.Set("X-Himate-Expected-Version", "0.8.25-start-23.11.3h")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if called {
		t.Fatal("mismatched release must be rejected before the application handler")
	}
	if !strings.Contains(rec.Body.String(), "RELEASE_MISMATCH") {
		t.Fatalf("expected RELEASE_MISMATCH, got %s", rec.Body.String())
	}
}

func TestReleaseGuardAllowsMatchingInternalMutation(t *testing.T) {
	t.Setenv("HIMATE_APP_VERSION", "0.8.26-start-23.11.3i")
	handler := ReleaseGuard("billing", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusCreated, map[string]any{"ok": true})
	}))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/billing/partners/ptr_000001/terms", strings.NewReader("{}"))
	req.Header.Set("X-Himate-Expected-Version", "0.8.26-start-23.11.3i")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}
