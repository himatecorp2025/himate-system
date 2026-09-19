package main

import "testing"

func TestLifecycleContract(t *testing.T) {
	for _, k := range []string{"PROSPECT", "LICENSE_PENDING", "READY_TO_PROVISION", "PROVISIONING", "CONFIGURATION", "TESTING", "READY_FOR_LAUNCH", "LIVE", "SUSPENDED", "ARCHIVED"} {
		if !lifecycleValues[k] {
			t.Fatalf("missing %s", k)
		}
	}
}
func TestSlugify(t *testing.T) {
	if got := slugify("Cultural Organization 42"); got != "cultural-organization-42" {
		t.Fatalf("got %s", got)
	}
}
