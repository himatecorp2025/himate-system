package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPhase4AuthorityHeadersAreStripped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://gateway/api/v1/profile", nil)
	for _, name := range untrustedAuthorityHeaders {
		req.Header.Set(name, "spoofed")
	}
	stripUntrustedAuthorityHeaders(req)
	for _, name := range untrustedAuthorityHeaders {
		if got := req.Header.Get(name); got != "" {
			t.Fatalf("%s was not stripped: %q", name, got)
		}
	}
}

func TestPhase4BrowserOriginPolicyKeepsNonBrowserClientsCompatible(t *testing.T) {
	crossSite := httptest.NewRequest(http.MethodPost, "https://himate.example/api/v1/auth/login", nil)
	crossSite.Host = "himate.example"
	crossSite.Header.Set("Origin", "https://evil.example")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	if browserMutationOriginAllowed(crossSite) {
		t.Fatal("cross-site browser mutation must be rejected")
	}

	cli := httptest.NewRequest(http.MethodPost, "http://gateway/api/v1/auth/login", nil)
	if !browserMutationOriginAllowed(cli) {
		t.Fatal("originless non-browser API clients must remain compatible")
	}
}

func TestPhase4TOTPVerificationWindow(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	at := time.Unix(1760000000, 0).UTC()
	code, err := totpCode(secret, at)
	if err != nil {
		t.Fatalf("totpCode: %v", err)
	}
	if !verifyTOTP(secret, code, at) {
		t.Fatal("current TOTP code rejected")
	}
	if verifyTOTP(secret, "000000", at) && code != "000000" {
		t.Fatal("unexpected invalid TOTP acceptance")
	}
}
