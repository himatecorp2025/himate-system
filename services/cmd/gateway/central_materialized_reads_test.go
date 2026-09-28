package main

import (
	"net/http/httptest"
	"testing"
)

func TestMaterializedImpactSummaryPreservesPeriodFilter(t *testing.T) {
	tenant := map[string]any{
		"impact_api": map[string]any{
			"items": []map[string]any{
				{
					"metric_key": "ci.evidence",
					"label": "Evidence CI",
					"label_en": "Evidence CI",
					"label_hu": "Evidence CI",
					"unit": "count",
					"aggregation": "SUM",
				},
			},
		},
		"impact_values_api": map[string]any{
			"items": []map[string]any{
				{
					"id": 2,
					"partner_id": "ptr_test",
					"metric_key": "ci.evidence",
					"label": "Evidence CI",
					"unit": "count",
					"period_start": "2026-09-01",
					"period_end": "2026-09-20",
					"numeric_value": float64(9),
				},
				{
					"id": 1,
					"partner_id": "ptr_test",
					"metric_key": "ci.evidence",
					"label": "Evidence CI",
					"unit": "count",
					"period_start": "2025-01-01",
					"period_end": "2025-01-31",
					"numeric_value": float64(100),
				},
			},
		},
	}

	req := httptest.NewRequest(
		"GET",
		"/api/v1/impact/summary?partner_id=ptr_test&period_start=2026-09-01&period_end=2026-09-20",
		nil,
	)
	got, err := materializedImpactSummary(tenant, req)
	if err != nil {
		t.Fatalf("materializedImpactSummary returned error: %v", err)
	}
	items := anyItems(got["items"])
	if len(items) != 1 {
		t.Fatalf("items = %#v, want exactly one metric", items)
	}
	if value, ok := items[0]["numeric_value"].(float64); !ok || value != 9 {
		t.Fatalf("numeric_value = %#v, want 9", items[0]["numeric_value"])
	}
	if observations := central10Int(items[0]["observations"]); observations != 1 {
		t.Fatalf("observations = %d, want 1", observations)
	}
	if end := central10String(items[0]["latest_period_end"]); end != "2026-09-20" {
		t.Fatalf("latest_period_end = %q, want 2026-09-20", end)
	}
}

func TestMaterializedEvidenceListPreservesFiltersAndPagination(t *testing.T) {
	items := []map[string]any{
		{
			"id": "evd_3",
			"partner_id": "ptr_test",
			"metric_key": "ci.evidence",
			"evidence_type": "PDF",
			"title": "Verified CI Evidence C",
			"description": "September evidence",
			"verification_status": "VERIFIED",
			"period_start": "2026-09-01",
			"period_end": "2026-09-20",
			"reports": []any{"rpt_1"},
		},
		{
			"id": "evd_2",
			"partner_id": "ptr_test",
			"metric_key": "ci.evidence",
			"evidence_type": "PDF",
			"title": "Verified CI Evidence B",
			"description": "September evidence",
			"verification_status": "VERIFIED",
			"period_start": "2026-09-01",
			"period_end": "2026-09-20",
			"reports": []any{},
		},
		{
			"id": "evd_1",
			"partner_id": "ptr_test",
			"metric_key": "ci.evidence",
			"evidence_type": "PDF",
			"title": "Verified CI Evidence A",
			"description": "September evidence",
			"verification_status": "VERIFIED",
			"period_start": "2026-09-01",
			"period_end": "2026-09-20",
			"reports": []any{},
		},
		{
			"id": "evd_old",
			"partner_id": "ptr_test",
			"metric_key": "ci.evidence",
			"evidence_type": "PDF",
			"title": "Historical evidence",
			"description": "Outside requested period",
			"verification_status": "VERIFIED",
			"period_start": "2025-01-01",
			"period_end": "2025-01-31",
			"reports": []any{},
		},
	}

	req := httptest.NewRequest(
		"GET",
		"/api/v1/evidence?partner_id=ptr_test&q=Verified%20CI&evidence_type=PDF&verification_status=VERIFIED&period_start=2026-09-01&period_end=2026-09-20&limit=1&offset=0",
		nil,
	)
	got, err := materializedEvidenceList(items, req)
	if err != nil {
		t.Fatalf("materializedEvidenceList returned error: %v", err)
	}
	if total := central10Int(got["total"]); total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if count := central10Int(got["count"]); count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if got["has_more"] != true {
		t.Fatalf("has_more = %#v, want true", got["has_more"])
	}

	outsideReq := httptest.NewRequest(
		"GET",
		"/api/v1/evidence?partner_id=ptr_test&period_start=2024-01-01&period_end=2024-01-31",
		nil,
	)
	outside, err := materializedEvidenceList(items, outsideReq)
	if err != nil {
		t.Fatalf("outside-period materializedEvidenceList returned error: %v", err)
	}
	if total := central10Int(outside["total"]); total != 0 {
		t.Fatalf("outside-period total = %d, want 0", total)
	}

	reportReq := httptest.NewRequest(
		"GET",
		"/api/v1/evidence?partner_id=ptr_test&report_id=rpt_1",
		nil,
	)
	reported, err := materializedEvidenceList(items, reportReq)
	if err != nil {
		t.Fatalf("report-filter materializedEvidenceList returned error: %v", err)
	}
	if total := central10Int(reported["total"]); total != 1 {
		t.Fatalf("report-filter total = %d, want 1", total)
	}
}


func TestCentralBrowserMaterializedReadSeparatesBrowserAndLegacyClients(t *testing.T) {
	browser := httptest.NewRequest("GET", "/api/v1/environments", nil)
	browser.Header.Set("X-Himate-Locale", "en")
	browser.Header.Set("X-Himate-Read-Model", "browser")
	if !centralBrowserMaterializedRead(browser) {
		t.Fatal("browser Central GET with the explicit read-model discriminator must use materialized CQRS")
	}

	localizedLegacy := httptest.NewRequest("GET", "/api/v1/environments", nil)
	localizedLegacy.Header.Set("X-Himate-Locale", "en")
	if centralBrowserMaterializedRead(localizedLegacy) {
		t.Fatal("localized legacy/smoke GET must not be mistaken for browser CQRS")
	}

	legacy := httptest.NewRequest("GET", "/api/v1/environments", nil)
	if centralBrowserMaterializedRead(legacy) {
		t.Fatal("legacy/smoke GET without browser discriminator must reach authoritative compatibility API")
	}

	write := httptest.NewRequest("POST", "/api/v1/environments", nil)
	write.Header.Set("X-Himate-Locale", "en")
	write.Header.Set("X-Himate-Read-Model", "browser")
	if centralBrowserMaterializedRead(write) {
		t.Fatal("mutations must never be intercepted by materialized read path")
	}
}

func TestReadModelGlobalTenantScopesSeparateGlobalAndPartnerMutations(t *testing.T) {
	global := []struct {
		reason      string
		wantModules bool
		wantPlans   bool
		wantDesign  bool
	}{
		{"/api/v1/modules/audit", true, false, false},
		{"/api/v1/modules/finance/audit", true, false, false},
		{"/api/v1/module-groups/client_operations/audit", true, false, false},
		{"/api/v1/billing/plans/STARTER/audit", false, true, false},
		{"/api/v1/cms/design/publish/audit", false, false, true},
	}
	for _, tc := range global {
		modules, plans, design := readModelGlobalTenantScopes(tc.reason)
		if modules != tc.wantModules || plans != tc.wantPlans || design != tc.wantDesign {
			t.Fatalf("global tenant scope %q = (%v,%v,%v), want (%v,%v,%v)",
				tc.reason, modules, plans, design, tc.wantModules, tc.wantPlans, tc.wantDesign)
		}
		if !readModelReasonRefreshesAllTenants(tc.reason) {
			t.Fatalf("global mutation %q must refresh all tenant read models", tc.reason)
		}
	}

	local := []string{
		"/api/v1/partners/ptr_1/modules/finance/audit",
		"/partner/api/v1/modules/finance/activate/audit",
		"/partner/api/v1/modules/finance/subscription/audit",
		"/api/v1/billing/partners/ptr_1/plan/audit",
		"/api/v1/cms/pages/page_1/audit",
	}
	for _, reason := range local {
		modules, plans, design := readModelGlobalTenantScopes(reason)
		if modules || plans || design || readModelReasonRefreshesAllTenants(reason) {
			t.Fatalf("partner-scoped mutation %q must not amplify into all-tenant refresh", reason)
		}
	}
}

