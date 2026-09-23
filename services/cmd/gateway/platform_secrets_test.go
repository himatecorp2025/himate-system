package main

import "testing"

func TestPlatformSecretDefinitionsAreWriteOnly(t *testing.T) {
	for _, definition := range platformSecretDefinitions {
		if definition.Key == "" || definition.Environment == "" || definition.Provider == "" || definition.Consumer == "" {
			t.Fatalf("incomplete secret definition: %#v", definition)
		}
		item := platformSecretMetadata(definition, true, "usr_test", "now")
		if value, exists := item["value"]; !exists || value != nil {
			t.Fatalf("secret metadata must never expose a raw value for %s", definition.Key)
		}
		if item["configured"] != true || item["status"] != "CONFIGURED" {
			t.Fatalf("configured metadata mismatch for %s: %#v", definition.Key, item)
		}
	}
}

func TestPlatformSecretDefinitionLookupRejectsUnknownKey(t *testing.T) {
	if _, ok := platformSecretDefinitionByKey("arbitrary_untrusted_secret"); ok {
		t.Fatal("arbitrary secret keys must not be accepted without an explicit definition")
	}
}
