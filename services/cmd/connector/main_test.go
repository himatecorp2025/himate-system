package main

import (
	"testing"
)

func TestConnectorTokenHashIsStableAndOneWay(t *testing.T) {
	token:="hmc_crd_abc_secret"
	a:=tokenHash(token)
	b:=tokenHash(token)
	if a!=b || a==token { t.Fatal("connector token hash contract failed") }
	if normalizeEnvironment("staging")!="STAGING" || normalizeEnvironment("invalid")!="" { t.Fatal("environment normalization failed") }
}

func TestConnectorTokenLookupTreatsBase64URLTokenAsOpaque(t *testing.T) {
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


func TestStart22RegistryIsCompleteAndUnique(t *testing.T) {
	if err := start22ValidateRegistry(); err != nil {
		t.Fatalf("START-22 registry invalid: %v", err)
	}
	if len(start22DatasetRegistry) != 38 {
		t.Fatalf("expected 38 Klavierhaus module datasets, got %d", len(start22DatasetRegistry))
	}
	if len(start22DatasetByKey) != 38 {
		t.Fatalf("expected 38 unique dataset keys, got %d", len(start22DatasetByKey))
	}
	for _, definition := range start22DatasetRegistry {
		if definition.ModuleKey == "" || definition.DatasetKey == "" || len(definition.AllowedFields) == 0 {
			t.Fatalf("invalid registry entry: %+v", definition)
		}
	}
}

func TestStart22SHA512AndHMACContract(t *testing.T) {
	data := map[string]any{"assets_usd": 10.0, "account_count": 4.0}
	checksum := start22DataChecksum(data)
	if len(checksum) != 128 {
		t.Fatalf("expected SHA-512 hex length 128, got %d", len(checksum))
	}
	if !start22ConstantHexEqual(checksum, start22DataChecksum(map[string]any{"account_count": 4.0, "assets_usd": 10.0})) {
		t.Fatal("map checksum must be canonical regardless of insertion order")
	}
	signature := start22Signature("hmc_test_secret", "2026-09-21T12:00:00Z", "nonce-12345678", checksum)
	if len(signature) != 128 {
		t.Fatalf("expected HMAC-SHA-512 hex length 128, got %d", len(signature))
	}
	if signature != start22Signature("hmc_test_secret", "2026-09-21T12:00:00Z", "nonce-12345678", checksum) {
		t.Fatal("START-22 signature must be deterministic")
	}
	if start22ConstantHexEqual(signature, start22Signature("different", "2026-09-21T12:00:00Z", "nonce-12345678", checksum)) {
		t.Fatal("different credential must not produce the same signature")
	}
}

func TestStart22DatasetValidationEnforcesAllowlistAndChecksum(t *testing.T) {
	item := start22DataItem{
		ModuleKey: "finance",
		DatasetKey: "finance.balance_sheet",
		SchemaVersion: 1,
		PeriodStart: "2026-09-21",
		PeriodEnd: "2026-09-21",
		Aggregation: "LATEST",
		Data: map[string]any{"assets_usd": 10.0, "liabilities_usd": 2.0, "equity_usd": 8.0, "account_count": 4.0},
		IdempotencyKey: "finance-test-0001",
	}
	item.SourceChecksum = start22DataChecksum(item.Data)
	definition, _, _, err := start22ValidateItem(&item)
	if err != nil {
		t.Fatalf("valid dataset rejected: %v", err)
	}
	if definition.DatasetKey != "finance.balance_sheet" {
		t.Fatalf("unexpected definition: %+v", definition)
	}

	bad := item
	bad.Data = map[string]any{"password_hash": "never"}
	bad.SourceChecksum = start22DataChecksum(bad.Data)
	if _, _, _, err := start22ValidateItem(&bad); err == nil {
		t.Fatal("non-allowlisted sensitive field must be rejected")
	}

	bad = item
	bad.SourceChecksum = start22SHA512Hex([]byte("tampered"))
	if _, _, _, err := start22ValidateItem(&bad); err == nil {
		t.Fatal("tampered data checksum must be rejected")
	}

	bad = item
	bad.IdempotencyKey = "x"
	if _, _, _, err := start22ValidateItem(&bad); err == nil {
		t.Fatal("unsafe/short idempotency key must be rejected")
	}
}

func TestStart22AggregateChecksumIsOrderIndependent(t *testing.T) {
	values := []string{
		start22SHA512Hex([]byte("one")),
		start22SHA512Hex([]byte("two")),
		start22SHA512Hex([]byte("three")),
	}
	reversed := []string{values[2], values[1], values[0]}
	if start22AggregateChecksum(values) != start22AggregateChecksum(reversed) {
		t.Fatal("reconciliation aggregate checksum must be independent of row order")
	}
}

func TestStart22MetricClassification(t *testing.T) {
	tests := map[string]string{
		"revenue_usd": "USD",
		"conversion_rate": "ratio",
		"average_cycle_hours": "hours",
		"average_response_minutes": "minutes",
		"storage_bytes": "bytes",
		"ticket_count": "count",
	}
	for field, want := range tests {
		if got := start22MetricUnit(field); got != want {
			t.Fatalf("%s => %s, want %s", field, got, want)
		}
	}
}
