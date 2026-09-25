package serviceauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testToken = "0123456789abcdef0123456789abcdef"

func TestSignVerifyAndAuthorityTamper(t *testing.T) {
	now := time.Unix(1760000000, 0).UTC()
	req := httptest.NewRequest(http.MethodPost, "http://service/internal/v1/demo?b=2&a=1", nil)
	req.Header.Set("X-Himate-Partner-ID", "ptr_123")
	req.Header.Set("X-Himate-User-ID", "usr_123")
	if err := Sign(req, testToken, "gateway", now); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	caller, err := Verify(req, testToken, now.Add(20*time.Second), 5*time.Minute)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if caller != "gateway" {
		t.Fatalf("caller=%q", caller)
	}
	req.Header.Set("X-Himate-Partner-ID", "ptr_other")
	if _, err := Verify(req, testToken, now, 5*time.Minute); err == nil {
		t.Fatal("expected authority header tampering to invalidate signature")
	}
}

func TestVerifyRejectsUnsignedAndStale(t *testing.T) {
	now := time.Unix(1760000000, 0).UTC()
	req := httptest.NewRequest(http.MethodGet, "http://service/internal/v1/demo", nil)
	if _, err := Verify(req, testToken, now, 5*time.Minute); err == nil {
		t.Fatal("expected unsigned request rejection")
	}
	if err := Sign(req, testToken, "billing", now.Add(-10*time.Minute)); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := Verify(req, testToken, now, 5*time.Minute); err == nil {
		t.Fatal("expected stale request rejection")
	}
}


func TestKnownCallerIncludesFrozenPhase3BProducers(t *testing.T) {
	for _, caller := range []string{"workshop", "scheduler"} {
		if !KnownCaller(caller) {
			t.Fatalf("Phase 3B canonical producer %q is not recognized by service auth", caller)
		}
		if err := Sign(httptest.NewRequest(http.MethodPost, "http://automation/internal/v1/automation/events", nil), testToken, caller, time.Now().UTC()); err != nil {
			t.Fatalf("Sign(%s): %v", caller, err)
		}
	}
}
