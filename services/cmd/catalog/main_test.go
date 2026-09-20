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
	want := map[string]int{"finance_invoicing": 3, "technical": 16, "marketing": 8, "website_events": 11}
	for k, n := range want {
		if groups[k] != n {
			t.Fatalf("%s expected %d got %d", k, n, groups[k])
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
