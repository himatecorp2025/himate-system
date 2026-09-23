package common

import "testing"

func TestPlatformSecretRoundTrip(t *testing.T) {
	master := "0123456789abcdef0123456789abcdef"
	plain := "sk_test_secret_value"
	encoded, err := EncryptPlatformSecret(master, plain)
	if err != nil {
		t.Fatal(err)
	}
	if encoded == "" || encoded == plain {
		t.Fatalf("secret was not encrypted: %q", encoded)
	}
	got, err := DecryptPlatformSecret(master, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("expected %q, got %q", plain, got)
	}
}

func TestPlatformSecretRejectsWrongMaster(t *testing.T) {
	encoded, err := EncryptPlatformSecret("0123456789abcdef0123456789abcdef", "whsec_test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptPlatformSecret("fedcba9876543210fedcba9876543210", encoded); err == nil {
		t.Fatal("expected decryption failure with a different master")
	}
}
