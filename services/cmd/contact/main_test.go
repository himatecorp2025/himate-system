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

func TestNormalizeLeadStatus(t *testing.T) {
	for _, value := range []string{"NEW", "in_progress", " contacted ", "CLOSED"} {
		if normalizeLeadStatus(value) == "" {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"", "OPEN", "SPAM", "123"} {
		if normalizeLeadStatus(value) != "" {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestEnvIntValueBounds(t *testing.T) {
	if got := envIntValue("", 50, 1, 200); got != 50 { t.Fatalf("default: %d", got) }
	if got := envIntValue("0", 50, 1, 200); got != 1 { t.Fatalf("minimum: %d", got) }
	if got := envIntValue("999", 50, 1, 200); got != 200 { t.Fatalf("maximum: %d", got) }
	if got := envIntValue("75", 50, 1, 200); got != 75 { t.Fatalf("value: %d", got) }
}


func TestCentral1ContactClassificationNormalization(t *testing.T) {
	if got := normalizeContactClassification(" piano_technology ", organizationTypes); got != "PIANO_TECHNOLOGY" {
		t.Fatalf("organization type normalization=%q", got)
	}
	if got := normalizeContactClassification("pricing_licensing", inquiryTopics); got != "PRICING_LICENSING" {
		t.Fatalf("inquiry topic normalization=%q", got)
	}
	if got := normalizeContactClassification("unknown", organizationTypes); got != "" {
		t.Fatalf("unknown organization type must be rejected, got %q", got)
	}
	if got := normalizeContactClassification("", inquiryTopics); got != "" {
		t.Fatalf("empty inquiry topic must be rejected, got %q", got)
	}
}
