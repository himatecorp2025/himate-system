package main

import "testing"

func TestNotificationCategoryClassification(t *testing.T) {
	cases := map[string]string{
		"COMMENT_ADDED":             "COMMENT",
		"DEADLINE_TOMORROW":         "DEADLINE",
		"CALENDAR_EVENT_CREATED":    "CALENDAR",
		"TASK_ASSIGNED":             "WORKFLOW",
		"PAYMENT_RETRY_SCHEDULED":   "BILLING",
		"SECURITY_LOGIN_DETECTED":   "SECURITY",
		"OTHER":                     "SYSTEM",
	}
	for eventType, want := range cases {
		if got := notificationCategory(eventType, "", ""); got != want {
			t.Fatalf("%s: got %s want %s", eventType, got, want)
		}
	}
}

func TestPartnerNotificationVisibility(t *testing.T) {
	event := notificationEvent{
		DeliveryScope: "PARTNER",
		PartnerID: "ptr_a",
		TargetUserID: "u_a",
		AudiencePermission: "billing.read",
		ModuleKey: "finance",
	}
	permissions := map[string]bool{"billing.read": true}
	modules := map[string]bool{"finance": true}
	if !eventVisible(event, "PARTNER", "ptr_a", "u_a", permissions, modules) {
		t.Fatal("expected own-tenant targeted notification to be visible")
	}
	if eventVisible(event, "PARTNER", "ptr_b", "u_a", permissions, modules) {
		t.Fatal("cross-tenant notification leaked")
	}
	if eventVisible(event, "PARTNER", "ptr_a", "u_b", permissions, modules) {
		t.Fatal("target-user notification leaked")
	}
	if eventVisible(event, "PARTNER", "ptr_a", "u_a", map[string]bool{}, modules) {
		t.Fatal("permission-scoped notification leaked")
	}
	if eventVisible(event, "PARTNER", "ptr_a", "u_a", permissions, map[string]bool{}) {
		t.Fatal("module-scoped notification leaked")
	}
}
