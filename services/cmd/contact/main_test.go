package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	l := &rateLimiter{hits: map[string][]time.Time{}}
	for i := 0; i < 3; i++ {
		if !l.allow("127.0.0.1", 3, time.Minute) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.allow("127.0.0.1", 3, time.Minute) {
		t.Fatal("fourth request should be rate limited")
	}
}

func TestClientIPUsesForwardedFor(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/public/contact", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.2")
	if got := clientIP(r); got != "203.0.113.10" {
		t.Fatalf("unexpected client ip %q", got)
	}
}
