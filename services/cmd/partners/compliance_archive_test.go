package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComplianceSourcesExcludeCredentialStores(t *testing.T) {
	for _, source := range complianceSources {
		full := source.Schema + "." + source.Table
		for _, forbidden := range []string{
			"identity.users",
			"identity.partner_users",
			"identity.sessions",
			"identity.mfa_challenges",
			"identity.platform_secrets",
			"payments.partner_profiles",
		} {
			if strings.EqualFold(full, forbidden) {
				t.Fatalf("credential-bearing table %s must not enter the seven-year Compliance Vault", full)
			}
		}
	}
}

func TestCompliancePayloadHashIsStable(t *testing.T) {
	payload := map[string]any{
		"partner_id": "ptr_test",
		"retention_years": 7,
		"rows": []any{map[string]any{"id": 1, "status": "ARCHIVED"}},
	}
	first, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if compliancePayloadHash(first) != compliancePayloadHash(second) {
		t.Fatal("canonical compliance payload hash is not stable")
	}
}
