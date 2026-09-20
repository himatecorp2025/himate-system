package main

import "testing"

func TestEnvironmentIdentityIsStable(t *testing.T) {
	if got:=envID("ptr_000042","STAGING");got!="env_staging_000042"{t.Fatalf("unexpected id %s",got)}
	if got:=slug("Gallery Company NYC");got!="gallery-company-nyc"{t.Fatalf("unexpected slug %s",got)}
}


func TestSTART20PublicHostnameValidation(t *testing.T) {
	valid := []string{"partner.example.com", "www.himate-system.com", "a-b.example.org"}
	for _, host := range valid {
		if !validPublicHostname(host) {
			t.Fatalf("expected valid public hostname: %s", host)
		}
	}
	invalid := []string{"localhost", "partner.local", "https://example.com", "example", "bad_host.example.com", ""}
	for _, host := range invalid {
		if validPublicHostname(host) {
			t.Fatalf("expected invalid public hostname: %s", host)
		}
	}
	if !isInternalHostname("partner.himate-staging.local") {
		t.Fatal("staging .local hostname must be treated as internal")
	}
}

func TestSTART20LaunchReadiness(t *testing.T) {
	ready := environment{
		Kind: "PRODUCTION",
		DeploymentStatus: "DEPLOYED",
		RuntimeStatus: "OK",
		ActiveRelease: "2026.09.21",
		DomainStatus: "VERIFIED",
		DNSStatus: "VERIFIED",
		TLSStatus: "VERIFIED",
	}
	if blockers := launchReadiness(ready); len(blockers) != 0 {
		t.Fatalf("expected launch-ready production, got blockers: %#v", blockers)
	}

	tests := []struct{
		name string
		change func(*environment)
	}{
		{"kind", func(e *environment){e.Kind="STAGING"}},
		{"deployment", func(e *environment){e.DeploymentStatus="FAILED"}},
		{"runtime", func(e *environment){e.RuntimeStatus="ERROR"}},
		{"release", func(e *environment){e.ActiveRelease=""}},
		{"domain", func(e *environment){e.DomainStatus="FAILED"}},
		{"dns", func(e *environment){e.DNSStatus="FAILED"}},
		{"tls", func(e *environment){e.TLSStatus="FAILED"}},
	}
	for _, tc := range tests {
		e := ready
		tc.change(&e)
		if blockers := launchReadiness(e); len(blockers) == 0 {
			t.Fatalf("%s must block launch", tc.name)
		}
	}
}
