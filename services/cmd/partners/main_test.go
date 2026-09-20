package main

import "testing"

func TestLifecycleContract(t *testing.T) {
	for _, k := range []string{"PROSPECT", "LICENSE_PENDING", "READY_TO_PROVISION", "PROVISIONING", "CONFIGURATION", "TESTING", "READY_FOR_LAUNCH", "LIVE", "SUSPENDED", "ARCHIVED"} {
		if !lifecycleValues[k] {
			t.Fatalf("missing %s", k)
		}
	}
}

func TestLifecycleTransitionContract(t *testing.T) {
	allowed := [][2]string{
		{"PROSPECT", "LICENSE_PENDING"},
		{"LICENSE_PENDING", "READY_TO_PROVISION"},
		{"READY_TO_PROVISION", "PROVISIONING"},
		{"PROVISIONING", "CONFIGURATION"},
		{"CONFIGURATION", "TESTING"},
		{"TESTING", "READY_FOR_LAUNCH"},
		{"READY_FOR_LAUNCH", "LIVE"},
		{"LIVE", "SUSPENDED"},
		{"SUSPENDED", "LIVE"},
	}
	for _, pair := range allowed {
		if !canTransition(pair[0], pair[1]) {
			t.Fatalf("expected transition %s -> %s", pair[0], pair[1])
		}
	}

	blocked := [][2]string{
		{"PROSPECT", "LIVE"},
		{"LICENSE_PENDING", "TESTING"},
		{"CONFIGURATION", "LIVE"},
		{"ARCHIVED", "LIVE"},
	}
	for _, pair := range blocked {
		if canTransition(pair[0], pair[1]) {
			t.Fatalf("unexpected transition %s -> %s", pair[0], pair[1])
		}
	}
}

func TestBoundedInt(t *testing.T) {
	if got := boundedInt("500", 100, 1, 200); got != 200 {
		t.Fatalf("expected capped 200 got %d", got)
	}
	if got := boundedInt("bad", 100, 1, 200); got != 100 {
		t.Fatalf("expected fallback 100 got %d", got)
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("Cultural Organization 42"); got != "cultural-organization-42" {
		t.Fatalf("got %s", got)
	}
}
