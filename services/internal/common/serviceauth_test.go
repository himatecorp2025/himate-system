package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalAuthSignedInternalRoute(t *testing.T) {
	t.Setenv("HIMATE_REQUIRE_SERVICE_SIGNATURE", "true")
	t.Setenv("HIMATE_APP_VERSION", "")
	token := "0123456789abcdef0123456789abcdef"
	handler := InternalAuth(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	unsigned := httptest.NewRequest(http.MethodGet, "http://service/internal/v1/demo", nil)
	unsigned.Header.Set("X-Himate-Internal-Token", token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, unsigned)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unsigned status=%d", rec.Code)
	}

	t.Setenv("HIMATE_SERVICE_CALLER_ID", "gateway")
	server := httptest.NewServer(handler)
	defer server.Close()
	signed, err := http.NewRequest(http.MethodGet, server.URL+"/internal/v1/demo", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	BindInternalRequest(signed, token)
	resp, err := DoInternal(server.Client(), signed)
	if err != nil {
		t.Fatalf("DoInternal: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("signed status=%d", resp.StatusCode)
	}
}
