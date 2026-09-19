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
