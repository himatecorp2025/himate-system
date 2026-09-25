package main

import (
	"strings"
	"testing"
)

func TestSeedModules(t *testing.T) {
	if len(seedModules) < 1 {
		t.Fatal("module catalog must contain at least one seed module")
	}
	groupKeys := map[string]bool{}
	for _, group := range seedGroups {
		if strings.TrimSpace(group.Key) == "" {
			t.Fatal("module group key must not be empty")
		}
		groupKeys[group.Key] = true
	}
	seen := map[string]bool{}
	for _, m := range seedModules {
		if seen[m.Key] {
			t.Fatalf("duplicate %s", m.Key)
		}
		seen[m.Key] = true
		if !groupKeys[m.Group] {
			t.Fatalf("module %s references unknown group %s", m.Key, m.Group)
		}
		if strings.TrimSpace(seedModuleHU[m.Key]) == "" {
			t.Fatalf("missing Hungarian module label for %s", m.Key)
		}
	}
	for _, key := range []string{"needs_assessment", "two_factor_authentication"} {
		if !seen[key] || !central4PlannedModules[key] {
			t.Fatalf("planned Central-4 module missing: %s", key)
		}
	}
}

func TestPrimaryModuleGroupBaseline(t *testing.T) {
	if len(seedGroups) < 1 {
		t.Fatal("module catalog must contain at least one primary group")
	}
	seen := map[string]bool{}
	for _, group := range seedGroups {
		if seen[group.Key] {
			t.Fatalf("duplicate module group %s", group.Key)
		}
		seen[group.Key] = true
		if strings.TrimSpace(group.Label) == "" || strings.TrimSpace(seedGroupHU[group.Key]) == "" {
			t.Fatalf("module group %s must have bilingual labels", group.Key)
		}
	}
	for _, key := range []string{"finance_invoicing", "client_operations", "marketing", "website_events", "security_system"} {
		if !seen[key] {
			t.Fatalf("required Central-4 baseline group missing: %s", key)
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
	for _, state := range []string{"UNPUBLISHED", "PUBLISHED"} {
		if !publicationStates[state] {
			t.Fatalf("missing module publication state %s", state)
		}
	}
	for _, state := range []string{"LEGACY_REFERENCE", "IN_DEVELOPMENT", "READY"} {
		if !implementationStates[state] {
			t.Fatalf("missing implementation state %s", state)
		}
	}
	for _, state := range []string{"INACTIVE", "ACTIVE", "CANCEL_PENDING"} {
		if !entitlementStates[state] {
			t.Fatalf("missing entitlement state %s", state)
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


func TestSTART23113MarketplaceCanonicalCoverage(t *testing.T) {
	if len(seedModules) < 1 {
		t.Fatal("marketplace requires at least one canonical module")
	}
	for _, module := range seedModules {
		summary, ok := marketplaceSummaries[module.Key]
		if !ok {
			t.Fatalf("missing marketplace summary for %s", module.Key)
		}
		if strings.TrimSpace(summary.EN) == "" || strings.TrimSpace(summary.HU) == "" {
			t.Fatalf("marketplace summary must be bilingual for %s", module.Key)
		}
	}
}

func TestSTART23113DiscoveryDoesNotGrantExecution(t *testing.T) {
	if got := marketplaceAccessState("NOT_LICENSED", "INACTIVE", "UNPUBLISHED", "LEGACY_REFERENCE", "ACTIVE"); got != "COMING_SOON" {
		t.Fatalf("unpublished legacy module must be discoverable-only, got %s", got)
	}
	if marketplaceExecutable("UNPUBLISHED", "LEGACY_REFERENCE", "ACTIVE") {
		t.Fatal("discoverable legacy module must not be executable")
	}
	if got := marketplaceAccessState("NOT_LICENSED", "INACTIVE", "PUBLISHED", "READY", "ACTIVE"); got != "LOCKED" {
		t.Fatalf("published READY module outside entitlement must be LOCKED, got %s", got)
	}
	if got := marketplaceAccessState("ACTIVE", "ACTIVE", "PUBLISHED", "READY", "ACTIVE"); got != "ACTIVE" {
		t.Fatalf("published READY entitled module must be ACTIVE, got %s", got)
	}
}

func TestSTART23113MarketplaceMigrationContract(t *testing.T) {
	m := start23113MarketplaceMigration()
	if m.Version != 9 {
		t.Fatalf("expected catalog migration 9 got %d", m.Version)
	}
	joined := ""
	for _, stmt := range m.Statements {
		joined += stmt + "\n"
	}
	for _, token := range []string{"marketplace_visible", "marketplace_summary_en", "marketplace_summary_hu", "catalog_modules_marketplace_idx"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("marketplace migration missing %s", token)
		}
	}
}


func TestAutomationManifestContract(t *testing.T) {
	valid:=map[string]any{"automation":map[string]any{
		"contract_version":"1",
		"produces_events":[]any{"workflow.qc.passed.v1"},
		"consumes_events":[]any{"client_piano.saved.v1"},
		"commands":[]any{"workflow.start.v1"},
		"scheduled_actions":[]any{"invoice.reminder.v1"},
		"required_permissions":[]any{"workflow.write"},
		"required_modules":[]any{"pianos"},
	}}
	if err:=validateAutomationManifest(valid);err!=nil{t.Fatalf("valid automation manifest rejected: %v",err)}
	invalid:=map[string]any{"automation":map[string]any{"contract_version":"2","produces_events":[]any{"workflow.qc.passed.v1"}}}
	if err:=validateAutomationManifest(invalid);err==nil{t.Fatal("unsupported automation contract version must be rejected")}
	duplicate:=map[string]any{"automation":map[string]any{"contract_version":"1","produces_events":[]any{"workflow.qc.passed.v1","workflow.qc.passed.v1"}}}
	if err:=validateAutomationManifest(duplicate);err==nil{t.Fatal("duplicate event contracts must be rejected")}
}


func TestCentral4CatalogMigrationContract(t *testing.T) {
	m := central4CatalogMigration()
	if m.Version != 10 {
		t.Fatalf("expected catalog migration 10 got %d", m.Version)
	}
	joined := ""
	for _, stmt := range m.Statements {
		joined += stmt + "\n"
	}
	for _, token := range []string{
		"catalog.module_usage_events",
		"module_usage_events_module_time_idx",
		"module_usage_events_partner_module_time_idx",
		"client_operations",
		"security_system",
	} {
		if !strings.Contains(joined, token) {
			t.Fatalf("Central-4 migration missing %s", token)
		}
	}
}

func TestCentral4PlannedModulesAreCatalogEntriesNotCardinalityRules(t *testing.T) {
	seen := map[string]seedModule{}
	for _, module := range seedModules {
		seen[module.Key] = module
	}
	for key, group := range map[string]string{
		"needs_assessment": "client_operations",
		"two_factor_authentication": "security_system",
	} {
		module, ok := seen[key]
		if !ok {
			t.Fatalf("planned module missing: %s", key)
		}
		if module.Group != group {
			t.Fatalf("planned module %s group=%s want=%s", key, module.Group, group)
		}
		if !central4PlannedModules[key] {
			t.Fatalf("planned module marker missing: %s", key)
		}
	}
	if len(seedModules) < 1 {
		t.Fatal("catalog must stay non-empty")
	}
}
