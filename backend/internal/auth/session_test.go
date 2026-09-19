package auth

import (
	"testing"
	"time"
)

func TestSessionIssueAndParse(t *testing.T) {
	manager, err := NewSessionManager("01234567890123456789012345678901", 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	token, err := manager.Issue("usr_1", "admin@example.com", "Admin", []string{"platform_admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(token, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "usr_1" || claims.Email != "admin@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestSessionExpiry(t *testing.T) {
	manager, _ := NewSessionManager("01234567890123456789012345678901", time.Hour)
	now := time.Unix(1700000000, 0)
	token, _ := manager.Issue("usr_1", "admin@example.com", "Admin", nil, now)
	if _, err := manager.Parse(token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expected expired session to fail")
	}
}
