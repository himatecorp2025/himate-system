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
