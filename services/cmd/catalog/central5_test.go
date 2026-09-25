package main

import "testing"

func TestCentral5UnlimitedEntitlementModeConstant(t *testing.T) {
	if catalogEntitlementModeUnlimited != "UNLIMITED" {
		t.Fatalf("unexpected unlimited entitlement mode %q", catalogEntitlementModeUnlimited)
	}
}

func TestCentral5CatalogMigrationContract(t *testing.T) {
	m := central5CatalogMigration()
	if m.Version != 11 {
		t.Fatalf("expected catalog migration 11 got %d", m.Version)
	}
	if len(m.Statements) < 2 {
		t.Fatal("Central-5 catalog migration must create entitlement policy storage")
	}
}
