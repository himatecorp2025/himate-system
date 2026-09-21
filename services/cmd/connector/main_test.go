package main

import (
	"bytes"
	"encoding/base64"
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


func TestStart22AES256GCMEnvelopeEncryption(t *testing.T) {
	keyV1 := []byte("0123456789abcdef0123456789abcdef")
	keyV2 := []byte("abcdef0123456789abcdef0123456789")
	ringV1 := start22Keyring{
		ActiveVersion: "v1",
		Keys: map[string][]byte{"v1": keyV1},
	}
	data := map[string]any{
		"active_user_count": 4.0,
		"admin_count": 1.0,
	}
	plaintext := start22CanonicalJSON(data)
	aad := start22RecordAAD("ptr_test", "STAGING", "operations.users", "users-test-0001")
	envelope, err := ringV1.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if envelope.KeyVersion != "v1" {
		t.Fatalf("unexpected key version: %s", envelope.KeyVersion)
	}
	if len(envelope.Ciphertext) <= len(plaintext) {
		t.Fatal("AES-GCM ciphertext should include authentication tag")
	}
	if bytes.Contains(envelope.Ciphertext, []byte("active_user_count")) {
		t.Fatal("ciphertext must not expose plaintext field names")
	}
	decrypted, err := ringV1.Decrypt(envelope, aad)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("round trip mismatch: %s != %s", decrypted, plaintext)
	}

	if _, err := ringV1.Decrypt(envelope, start22RecordAAD("ptr_other", "STAGING", "operations.users", "users-test-0001")); err == nil {
		t.Fatal("AAD must bind encrypted data to partner/environment/dataset/idempotency identity")
	}

	tampered := envelope
	tampered.Ciphertext = append([]byte(nil), envelope.Ciphertext...)
	tampered.Ciphertext[0] ^= 0x01
	if _, err := ringV1.Decrypt(tampered, aad); err == nil {
		t.Fatal("tampered ciphertext must fail AES-GCM authentication")
	}

	ringV2 := start22Keyring{
		ActiveVersion: "v2",
		Keys: map[string][]byte{"v1": keyV1, "v2": keyV2},
	}
	if _, err := ringV2.Decrypt(envelope, aad); err != nil {
		t.Fatalf("versioned keyring must decrypt retained records after rotation: %v", err)
	}
	newEnvelope, err := ringV2.Encrypt(plaintext, aad)
	if err != nil || newEnvelope.KeyVersion != "v2" {
		t.Fatalf("new writes must use active rotated key: %+v err=%v", newEnvelope, err)
	}
}

func TestStart22AES256MasterKeyValidation(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	key, err := start22DecodeAES256Key(valid)
	if err != nil || len(key) != 32 {
		t.Fatalf("valid AES-256 key rejected: len=%d err=%v", len(key), err)
	}
	invalid := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := start22DecodeAES256Key(invalid); err == nil {
		t.Fatal("non-256-bit connector encryption key must be rejected")
	}
}
