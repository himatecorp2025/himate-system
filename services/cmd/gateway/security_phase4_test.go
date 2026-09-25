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

func TestSTART241AdminMFAVerifyHonorsAuthenticationThrottle(t *testing.T) {
	a := &app{loginAttempts: map[string]loginState{}}
	req := httptest.NewRequest(http.MethodPost, "https://himate.example/api/v1/auth/mfa/verify", nil)
	req.Host = "himate.example"
	req.RemoteAddr = "203.0.113.41:43123"
	key := clientKey(req)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		a.recordLoginFailure(key, now.Add(time.Duration(i)*time.Millisecond))
	}
	rec := httptest.NewRecorder()
	a.adminMFAVerify(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("admin MFA throttle status=%d want=%d body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

func TestSTART241PartnerMFAVerifyHonorsAuthenticationThrottle(t *testing.T) {
	a := &app{loginAttempts: map[string]loginState{}}
	req := httptest.NewRequest(http.MethodPost, "https://himate.example/partner/api/v1/auth/mfa/verify", nil)
	req.Host = "himate.example"
	req.RemoteAddr = "203.0.113.42:43124"
	key := "partner:" + clientKey(req)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		a.recordLoginFailure(key, now.Add(time.Duration(i)*time.Millisecond))
	}
	rec := httptest.NewRecorder()
	a.partnerMFAVerify(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("partner MFA throttle status=%d want=%d body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}



func TestCentral1PartnerMFAIsOptionalUntilModule40Activation(t *testing.T) {
	for _, role := range []string{"owner", "admin", "billing", "viewer"} {
		if partnerMFARequired(role) {
			t.Fatalf("partner role %q must not require MFA before optional module 40 is activated", role)
		}
	}
}
