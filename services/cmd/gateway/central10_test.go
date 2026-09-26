package main

import (
	"testing"
	"time"
)

func TestCentral10CanonicalPlans(t *testing.T) {
	cases := []struct {
		key, name, display, entitlement string
		price                         int
		limit                         any
	}{
		{"STARTER", "Starter", "$990 + VAT", "10 modules", 990, 10},
		{"BUSINESS", "Business", "$1,490 + VAT", "20 modules", 1490, 20},
		{"FLEX", "Premium", "$2,490 + VAT", "Unlimited", 2490, nil},
	}
	for _, tc := range cases {
		got := central10CanonicalPlan(map[string]any{"plan_key": tc.key}, map[string]map[string]any{})
		if got["display_name"] != tc.name {
			t.Fatalf("%s display name = %v", tc.key, got["display_name"])
		}
		if got["display_price"] != tc.display {
			t.Fatalf("%s display price = %v", tc.key, got["display_price"])
		}
		if got["monthly_price"] != tc.price {
			t.Fatalf("%s monthly price = %v", tc.key, got["monthly_price"])
		}
		if got["entitlement"] != tc.entitlement {
			t.Fatalf("%s entitlement = %v", tc.key, got["entitlement"])
		}
		if tc.limit == nil {
			if got["module_limit"] != nil {
				t.Fatalf("%s module limit = %v, want nil", tc.key, got["module_limit"])
			}
		} else if got["module_limit"] != tc.limit {
			t.Fatalf("%s module limit = %v, want %v", tc.key, got["module_limit"], tc.limit)
		}
	}
}

func TestCentral10ImpactEmptyDataDoesNotFabricatePoints(t *testing.T) {
	got := central10NormalizeDashboardImpact(map[string]any{
		"has_data":          false,
		"observation_count": 0,
		"trend":             []any{map[string]any{"value": 0}},
		"weekly_trend":      []any{map[string]any{"value": 0}},
	})
	if got["has_data"] != false {
		t.Fatalf("has_data = %v", got["has_data"])
	}
	if rows := anyItems(got["trend"]); len(rows) != 0 {
		t.Fatalf("empty dataset produced monthly rows: %#v", rows)
	}
	if rows := anyItems(got["weekly_trend"]); len(rows) != 0 {
		t.Fatalf("empty dataset produced weekly rows: %#v", rows)
	}
}

func TestCentral10WeeklyWindowKeepsLatestFourElapsedWeeks(t *testing.T) {
	now := time.Now().UTC()
	currentMonday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(int(now.Weekday())+6)%7)

	rows := make([]any, 0, 10)
	for delta := -6; delta <= 3; delta++ {
		week := currentMonday.AddDate(0, 0, delta*7)
		rows = append(rows, map[string]any{
			"week_start": week.Format("2006-01-02"),
			"value":      delta + 10,
		})
	}
	got := central10NormalizeDashboardImpact(map[string]any{
		"has_data":          true,
		"observation_count": 10,
		"trend":             []any{map[string]any{"label": "Sep", "value": 1}},
		"weekly_trend":      rows,
	})
	weekly := anyItems(got["weekly_trend"])
	if len(weekly) != 4 {
		t.Fatalf("weekly rows = %d, want 4: %#v", len(weekly), weekly)
	}
	first, _ := time.Parse("2006-01-02", central10String(weekly[0]["week_start"]))
	last, _ := time.Parse("2006-01-02", central10String(weekly[len(weekly)-1]["week_start"]))
	if !first.Equal(currentMonday.AddDate(0, 0, -21)) {
		t.Fatalf("first week = %s, want %s", first, currentMonday.AddDate(0, 0, -21))
	}
	if !last.Equal(currentMonday) {
		t.Fatalf("last week = %s, want %s", last, currentMonday)
	}
}


func TestCentral10CommercialLimitHasNoTotalDatasetCeiling(t *testing.T) {
	for _, raw := range []string{"201", "500", "1000"} {
		got := central10PositiveInt(raw, 120)
		want := 0
		for _, ch := range raw {
			want = want*10 + int(ch-'0')
		}
		if got != want {
			t.Fatalf("central10PositiveInt(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestCentral10CommercialChunkingPreservesMoreThanTwoHundredPartners(t *testing.T) {
	ids := make([]string, 0, 205)
	for i := 0; i < 205; i++ {
		ids = append(ids, "partner")
	}
	chunks := central10StringChunks(ids, 80)
	total := 0
	for _, chunk := range chunks {
		if len(chunk) > 80 {
			t.Fatalf("chunk size = %d, want <= 80", len(chunk))
		}
		total += len(chunk)
	}
	if total != len(ids) {
		t.Fatalf("chunked total = %d, want %d", total, len(ids))
	}
	if len(chunks) != 3 {
		t.Fatalf("chunk count = %d, want 3", len(chunks))
	}
}


func TestCentral10PartnerModuleViewOwnsFilteringGroupingKPIsAndSubscriptionJoin(t *testing.T) {
	modules := []map[string]any{
		{"key": "finance.invoice", "label": "Invoices", "group_key": "finance_invoicing", "group_label": "Finance", "status": "ACTIVE", "included_in_base": true},
		{"key": "marketing.crm", "label": "CRM", "group_key": "marketing", "group_label": "Marketing", "status": "MAINTENANCE", "included_in_base": false},
		{"key": "website.events", "label": "Events", "group_key": "website_events", "group_label": "Website", "status": "NOT_LICENSED", "included_in_base": false},
	}
	subscriptions := []map[string]any{
		{"module_key": "finance.invoice", "cancel_at_period_end": false, "period_end_exclusive": "2026-10-01"},
	}
	view := central10PartnerModuleView(modules, subscriptions, "invoice", "ACTIVE")

	kpis, ok := view["kpis"].(map[string]any)
	if !ok {
		t.Fatalf("kpis type = %T", view["kpis"])
	}
	if central10Int(kpis["total"]) != 3 || central10Int(kpis["active"]) != 1 ||
		central10Int(kpis["maintenance"]) != 1 || central10Int(kpis["base_included"]) != 1 {
		t.Fatalf("unexpected module KPIs: %#v", kpis)
	}
	filtered := anyItems(view["filtered_items"])
	if len(filtered) != 1 || filtered[0]["key"] != "finance.invoice" {
		t.Fatalf("filtered_items = %#v", filtered)
	}
	if _, ok := filtered[0]["subscription"].(map[string]any); !ok {
		t.Fatalf("subscription was not joined: %#v", filtered[0])
	}
	groups := anyItems(view["groups"])
	if len(groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(groups))
	}
	if central10Int(groups[0]["count"]) != 1 || groups[0]["label"] != "Finance & Invoicing" {
		t.Fatalf("first group = %#v", groups[0])
	}
	keys, ok := view["active_module_keys"].([]string)
	if !ok || len(keys) != 1 || keys[0] != "finance.invoice" {
		t.Fatalf("active_module_keys = %#v", view["active_module_keys"])
	}
}

func TestCentral10FinanceChartOwnsPeriodPlanCurrencySelection(t *testing.T) {
	overview := map[string]any{
		"currencies": []map[string]any{
			{"currency": "USD"},
			{"currency": "EUR"},
		},
		"weekly_paid_by_plan": []map[string]any{
			{"period": "2026-09-07", "currency": "USD", "plan_key": "STARTER", "paid": 120.0},
			{"period": "2026-09-14", "currency": "USD", "plan_key": "STARTER", "paid": 180.0},
			{"period": "2026-09-14", "currency": "USD", "plan_key": "BUSINESS", "paid": 900.0},
			{"period": "2026-09-14", "currency": "EUR", "plan_key": "STARTER", "paid": 500.0},
		},
	}
	chart := central10FinanceChart(overview, "WEEKLY", "STARTER", "USD")
	if chart["period"] != "WEEKLY" || chart["plan_key"] != "STARTER" || chart["currency"] != "USD" {
		t.Fatalf("chart selector = %#v", chart)
	}
	rows := anyItems(chart["rows"])
	if len(rows) != 2 {
		t.Fatalf("chart rows = %#v", rows)
	}
	if central10Float(chart["max_paid"]) != 180 {
		t.Fatalf("max_paid = %v", chart["max_paid"])
	}
}

func TestCentral10MoneyLabelIsReadyToRender(t *testing.T) {
	rows := []map[string]any{
		{"currency": "USD", "outstanding": 125.5},
		{"currency": "EUR", "outstanding": 0.0},
	}
	if got := central10MoneyLabel(rows, "outstanding"); got != "USD 125.50" {
		t.Fatalf("single currency label = %q", got)
	}
	rows[1]["outstanding"] = 5.0
	if got := central10MoneyLabel(rows, "outstanding"); got != "2 currencies" {
		t.Fatalf("multi-currency label = %q", got)
	}
}

func TestCentral10PartnerCategoriesAreBackendMergedAndOrdered(t *testing.T) {
	got := central10PartnerCategories("hu_HU", []map[string]any{
		{"id": "cat_002", "name": "Remote Fine Art", "name_en": "Remote Fine Art", "name_hu": "Távoli művészet"},
		{"id": "custom_z", "name": "Zulu"},
		{"id": "custom_a", "name": "Alpha"},
	})
	if len(got) != 8 {
		t.Fatalf("categories = %d, want 8: %#v", len(got), got)
	}
	if got[0]["id"] != "cat_001" || got[1]["id"] != "cat_002" || got[6]["id"] != "custom_a" || got[7]["id"] != "custom_z" {
		t.Fatalf("category order = %#v", got)
	}
	if got[1]["name"] != "Remote Fine Art" {
		t.Fatalf("remote category override lost: %#v", got[1])
	}
}
