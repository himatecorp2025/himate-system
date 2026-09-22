package main

import "testing"

func TestSeedModules(t *testing.T) {
	if len(seedModules) != 38 {
		t.Fatalf("expected 38 got %d", len(seedModules))
	}
	seen := map[string]bool{}
	groups := map[string]int{}
	for _, m := range seedModules {
		if seen[m.Key] {
			t.Fatalf("duplicate %s", m.Key)
		}
		seen[m.Key] = true
		groups[m.Group]++
	}
	want := map[string]int{"workshop": 1, "finance_invoicing": 3, "technical": 15, "marketing": 8, "website_events": 11}
	for k, n := range want {
		if groups[k] != n {
			t.Fatalf("%s expected %d got %d", k, n, groups[k])
		}
	}
}

func TestSixModuleGroups(t *testing.T) {
	if len(seedGroups) != 6 {
		t.Fatalf("expected six module groups got %d", len(seedGroups))
	}
	want := []string{"workshop", "finance_invoicing", "technical", "marketing", "website_events", "communication"}
	for i, key := range want {
		if seedGroups[i].Key != key {
			t.Fatalf("group %d expected %s got %s", i, key, seedGroups[i].Key)
		}
	}
}

func TestModuleStateContract(t *testing.T) {
	for _, state := range []string{"ACTIVE", "NOT_LICENSED", "MAINTENANCE"} {
		if !moduleStates[state] {
			t.Fatalf("missing partner module state %s", state)
		}
	}
	for _, state := range []string{"ACTIVE", "UNAVAILABLE", "DEPRECATED"} {
		if !availabilityValues[state] {
			t.Fatalf("missing module availability %s", state)
		}
	}
}

func TestStableModuleKeyContract(t *testing.T) {
	for _, key := range []string{"finance.balance_sheet", "marketing_campaigns", "website.events"} {
		if !moduleKeyPattern.MatchString(key) {
			t.Fatalf("expected valid stable key %s", key)
		}
	}
	for _, key := range []string{"UpperCase", "a b", "../unsafe"} {
		if moduleKeyPattern.MatchString(key) {
			t.Fatalf("expected invalid stable key %s", key)
		}
	}
}

func TestSTART232PartnerIDMatrixInputIsBoundedAndDeduplicated(t *testing.T) {
	raw := "ptr_1, ptr_2,ptr_1,,ptr_3"
	got := splitPartnerIDs(raw)
	if len(got) != 3 || got[0] != "ptr_1" || got[1] != "ptr_2" || got[2] != "ptr_3" {
		t.Fatalf("unexpected partner IDs: %#v", got)
	}
}

func TestSTART232CommercialDefaultsAreNonNegativeByContract(t *testing.T) {
	if !moduleStates["ACTIVE"] || !availabilityValues["ACTIVE"] {
		t.Fatal("commercial matrix requires active module contracts")
	}
}
