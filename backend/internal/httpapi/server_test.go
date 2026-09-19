package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"himate.local/backend/internal/auth"
	"himate.local/backend/internal/config"
	"himate.local/backend/internal/model"
	"himate.local/backend/internal/security"
	"himate.local/backend/internal/service"
	"himate.local/backend/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	users := store.NewMemoryUserStore()
	hash, err := security.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.CreateUser(model.User{
		ID: "usr_1", Name: "HIMATE Admin", Email: "admin@himate.test",
		PasswordHash: hash, Roles: []string{"platform_admin"}, Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	sessions, _ := auth.NewSessionManager("01234567890123456789012345678901", 8*time.Hour)
	return NewServer(config.Config{
		Environment: "test",
		AppVersion:  "test",
		WebDistDir:  t.TempDir(),
		SessionTTL:  8 * time.Hour,
	}, service.NewAuthService(users), sessions, slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)))
}

func TestHealth(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestLoginAndAuthenticatedSummary(t *testing.T) {
	server := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"email":    "admin@himate.test",
		"password": "correct-horse-battery-staple",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRes.Code, loginRes.Body.String())
	}

	cookies := loginRes.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	summaryReq.AddCookie(cookies[0])
	summaryRes := httptest.NewRecorder()
	server.Handler().ServeHTTP(summaryRes, summaryReq)
	if summaryRes.Code != http.StatusOK {
		t.Fatalf("expected summary 200, got %d: %s", summaryRes.Code, summaryRes.Body.String())
	}
}

func TestSummaryRequiresAuthentication(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}
