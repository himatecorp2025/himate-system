package main

import "testing"

func TestConnectorTokenHashIsStableAndOneWay(t *testing.T) {
	token:="hmc_crd_abc_secret"
	a:=tokenHash(token)
	b:=tokenHash(token)
	if a!=b || a==token { t.Fatal("connector token hash contract failed") }
	if normalizeEnvironment("staging")!="STAGING" || normalizeEnvironment("invalid")!="" { t.Fatal("environment normalization failed") }
}

func TestConnectorTokenLookupTreatsBase64URLTokenAsOpaque(t *testing.T) {
	// '_' is part of the Base64URL alphabet and may legitimately occur in
	// either random segment. Authentication must therefore hash the complete
	// token instead of splitting it on underscores.
	token := "hmc_crd_ab_cd_ef_secret_with_many_under_scores"
	got, err := connectorTokenLookupHash(token)
	if err != nil {
		t.Fatalf("opaque connector token rejected: %v", err)
	}
	if got != tokenHash(token) {
		t.Fatal("connector lookup must use the hash of the complete token")
	}

	if _, err := connectorTokenLookupHash("invalid_token"); err == nil {
		t.Fatal("invalid connector token prefix must be rejected")
	}
	if _, err := connectorTokenLookupHash(""); err == nil {
		t.Fatal("empty connector token must be rejected")
	}
}
