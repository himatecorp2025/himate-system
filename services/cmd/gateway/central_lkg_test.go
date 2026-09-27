package main

import (
	"context"
	"testing"
	"time"
)

func validRegistrySnapshot(label string) map[string]any {
	return map[string]any{
		"status":      "healthy",
		"unavailable": []string{},
		"modules":     []map[string]any{{"key": "test.module", "label": label}},
		"groups":      []map[string]any{},
		"trend":       []map[string]any{},
		"module_details": map[string]any{
			"test.module": map[string]any{
				"relationships": map[string]any{"items": []map[string]any{}, "count": 0},
				"impact_metrics": map[string]any{"items": []map[string]any{}, "count": 0},
				"usage": map[string]any{"items": []map[string]any{}, "count": 0, "usage_summary": map[string]any{}},
			},
		},
	}
}

func validAdministrationSnapshot() map[string]any {
	return map[string]any{
		"status":      "healthy",
		"unavailable": []string{},
		"company":           map[string]any{},
		"items":             []map[string]any{},
		"kpis":              map[string]any{},
		"admin_roles":       map[string]any{},
		"admin_users":       map[string]any{},
		"admin_secrets":     map[string]any{},
		"audit_events":      map[string]any{},
		"company_documents": map[string]any{},
		"invoice_register":  map[string]any{},
		"backup_api":        map[string]any{},
	}
}

func validSystemSnapshot() map[string]any {
	return map[string]any{
		"status":      "healthy",
		"unavailable": []string{},
		"health":      map[string]any{},
		"provisioning": []map[string]any{},
		"environments": []map[string]any{},
		"events":       []map[string]any{},
		"backups":      map[string]any{},
		"kpis":         map[string]any{},
		"health_api":        map[string]any{},
		"provisioning_api":  map[string]any{},
		"environments_api":  map[string]any{},
		"backups_api":              map[string]any{},
		"backup_restore_points":    map[string]any{},
		"backup_restore_tests_api": map[string]any{"items": []map[string]any{}},
		"backup_restore_jobs_api":  map[string]any{"items": []map[string]any{}},
	}
}

func validPartnerWorkspaceSnapshot() map[string]any {
	return map[string]any{
		"status":                          "healthy",
		"unavailable":                     []string{},
		"partner":                         map[string]any{"id": "partner_1"},
		"modules":                         []map[string]any{},
		"module_view":                     map[string]any{},
		"production_environment":          nil,
		"preferred_connector_environment": "STAGING",
		"billing":                         map[string]any{},
		"terms":                           map[string]any{},
		"license":                         map[string]any{},
		"documents":                       []map[string]any{},
		"invoices":                        []map[string]any{},
		"subscriptions":                   []map[string]any{},
		"environments":                    []map[string]any{},
		"provisioning_jobs":               []map[string]any{},
		"impact_summary":                  []map[string]any{},
		"evidence":                        []map[string]any{},
		"connector_credentials":           []map[string]any{},
		"portal_users":                    []map[string]any{},
		"agreement":                       map[string]any{},
		"commercial_status":               map[string]any{},
		"billing_events":                  []map[string]any{},
		"website_adapter":                 map[string]any{},
		"partner_design":                  map[string]any{},
		"payment_profile":                 map[string]any{},
		"catalog_modules_api":             map[string]any{"items": []map[string]any{}},
		"environments_api":                map[string]any{"items": []map[string]any{}},
		"provisioning_api":                map[string]any{"items": []map[string]any{}},
		"impact_api":                      map[string]any{"items": []map[string]any{}},
		"evidence_api":                    map[string]any{"items": []map[string]any{}},
		"connector_credentials_api":       map[string]any{"items": []map[string]any{}},
		"portal_users_api":                map[string]any{"items": []map[string]any{}, "count": 0},
		"portal_gate":                     map[string]any{},
		"portal_modules":                  map[string]any{},
		"portal_plans":                    map[string]any{},
		"portal_plan":                     map[string]any{},
		"portal_plan_modules":             map[string]any{},
		"portal_charity":                  map[string]any{},
		"portal_charity_modules":          map[string]any{},
		"portal_design_media":             map[string]any{},
		"portal_billing_subscriptions":    map[string]any{},
		"portal_billing_invoices":         map[string]any{},
		"portal_user_module_policies":     map[string]any{},
		"portal_notifications":            map[string]any{},
		"tenant_finance":                  map[string]any{},
		"partner_audit_events":            []map[string]any{},
		"partner_contacts":                []map[string]any{},
		"partner_domains_deployments":     map[string]any{"items": []map[string]any{}, "count": 0},
		"partner_permissions":             map[string]any{"users": []map[string]any{}, "module_policies": map[string]any{}},
		"module_commercial_history":       map[string]any{"items": map[string]any{}, "count": 0},
		"start22_summary":                  map[string]any{"ALL": map[string]any{}, "PRODUCTION": map[string]any{}, "STAGING": map[string]any{}},
		"start22_retention":                map[string]any{},
	}
}

func validPlansSnapshot() map[string]any {
	return map[string]any{
		"status": "healthy", "unavailable": []string{},
		"plans": []map[string]any{},
		"modules": []map[string]any{},
	}
}

func validAnalyticsSnapshot() map[string]any {
	return map[string]any{
		"status": "healthy", "unavailable": []string{},
		"analytics": map[string]any{},
		"modules": []map[string]any{},
	}
}

func validCommercialSnapshot() map[string]any {
	return map[string]any{
		"status": "healthy", "unavailable": []string{},
		"partners": []map[string]any{},
		"matrix_items": []map[string]any{},
		"subscription_items": []map[string]any{},
		"modules": []map[string]any{},
		"plans": []map[string]any{},
		"matrix_available": true,
		"subscriptions_available": true,
	}
}

func validGlobalSearchSnapshot() map[string]any {
	return map[string]any{
		"status": "healthy", "unavailable": []string{},
		"partners": []map[string]any{},
		"modules": []map[string]any{},
		"contact_inquiries": []map[string]any{},
		"cms_pages": []map[string]any{},
		"admin_users": []map[string]any{},
		"audit_events": []map[string]any{},
	}
}

func TestCentralSnapshotValidRejectsDegraded(t *testing.T) {
	for _, status := range []string{"partial", "unavailable", "warming", "stale", ""} {
		payload := validRegistrySnapshot("LKG")
		payload["status"] = status
		if centralSnapshotValid(centralStep3RegistryKey, payload) {
			t.Fatalf("status %q was accepted as Last-Known-Good", status)
		}
	}
}

func TestCentralSnapshotValidRejectsHiddenUnavailable(t *testing.T) {
	payload := validRegistrySnapshot("LKG")
	payload["unavailable"] = []string{"catalog"}
	if centralSnapshotValid(centralStep3RegistryKey, payload) {
		t.Fatal("healthy snapshot with hidden unavailable dependency was accepted")
	}
	tenant := validPartnerWorkspaceSnapshot()
	tenant["unavailable"] = []string{"billing"}
	if partnerWorkspaceSnapshotValid(tenant) {
		t.Fatal("healthy tenant snapshot with hidden unavailable dependency was accepted")
	}
}

func TestCentralSnapshotValidRejectsLegacyNonCompositeScreens(t *testing.T) {
	plans := validPlansSnapshot()
	delete(plans, "modules")
	if centralSnapshotValid(centralStep3PlansKey, plans) {
		t.Fatal("legacy Plans snapshot without module projection was accepted")
	}

	analytics := validAnalyticsSnapshot()
	delete(analytics, "modules")
	if centralSnapshotValid(centralStep3AnalyticsKey, analytics) {
		t.Fatal("legacy package analytics snapshot without module projection was accepted")
	}

	commercial := validCommercialSnapshot()
	delete(commercial, "plans")
	if centralSnapshotValid(centralStep3CommercialKey, commercial) {
		t.Fatal("legacy Commercial snapshot without plans projection was accepted")
	}
}

func TestCentralSnapshotValidRejectsMissingRequiredField(t *testing.T) {
	payload := validRegistrySnapshot("LKG")
	delete(payload, "modules")
	if centralSnapshotValid(centralStep3RegistryKey, payload) {
		t.Fatal("registry without modules was accepted as Last-Known-Good")
	}
}

func TestCentralSnapshotValidAcceptsAuthoritativeScreens(t *testing.T) {
	cases := []struct {
		key     string
		payload map[string]any
	}{
		{centralStep3RegistryKey, validRegistrySnapshot("Registry")},
		{centralStep3PlansKey, validPlansSnapshot()},
		{centralStep3AnalyticsKey, validAnalyticsSnapshot()},
		{centralStep3CommercialKey, validCommercialSnapshot()},
		{centralStep4GlobalSearchKey, validGlobalSearchSnapshot()},
		{centralStep4AdministrationKey, validAdministrationSnapshot()},
		{centralStep4SystemKey, validSystemSnapshot()},
		{centralPartnerWorkspaceKey("partner_1"), validPartnerWorkspaceSnapshot()},
	}
	for _, tc := range cases {
		if !centralSnapshotValid(tc.key, tc.payload) {
			t.Fatalf("healthy authoritative snapshot %q was rejected: %#v", tc.key, tc.payload)
		}
	}
}

func TestCentralStep3StoreRetainsLastKnownGood(t *testing.T) {
	centralStep3Snapshots.Lock()
	originalItems := centralStep3Snapshots.items
	centralStep3Snapshots.items = map[string]centralStep3SnapshotEntry{}
	centralStep3Snapshots.Unlock()
	defer func() {
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items = originalItems
		centralStep3Snapshots.Unlock()
	}()

	lkg := validRegistrySnapshot("Last Known Good")
	centralStep3Snapshots.Lock()
	centralStep3Snapshots.items[centralStep3RegistryKey] = centralStep3SnapshotEntry{
		payload:   centralStep3CopyMap(lkg),
		updatedAt: time.Now().UTC(),
	}
	centralStep3Snapshots.Unlock()

	degraded := validRegistrySnapshot("Broken refresh")
	degraded["status"] = "partial"
	degraded["unavailable"] = []string{"catalog"}
	degraded["modules"] = []map[string]any{}

	a := &app{}
	a.centralStep3Store(context.Background(), centralStep3RegistryKey, degraded)

	got, _, ok := centralStep3SnapshotGet(centralStep3RegistryKey)
	if !ok {
		t.Fatal("Last-Known-Good snapshot disappeared after degraded refresh")
	}
	modules := anyItems(got["modules"])
	if len(modules) != 1 || central10String(modules[0]["label"]) != "Last Known Good" {
		t.Fatalf("degraded refresh replaced Last-Known-Good snapshot: %#v", got)
	}
	if central10String(got["status"]) != "healthy" {
		t.Fatalf("retained snapshot status = %q, want healthy", central10String(got["status"]))
	}
}
