package main

import (
	"testing"
	"time"
)

func TestJanuaryFirstIncrease(t *testing.T) {
	x := terms{BaseMonthlyFee: 2000, AnnualIncreasePercent: 10, PriceEffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if got := effectiveBaseFee(x, time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)); got != 2000 {
		t.Fatalf("2026 got %.2f", got)
	}
	if got := effectiveBaseFee(x, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)); got != 2200 {
		t.Fatalf("2027 got %.2f", got)
	}
	if got := effectiveBaseFee(x, time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)); got != 2420 {
		t.Fatalf("2028 got %.2f", got)
	}
}

func TestCycleWindowIsActivationAnchored(t *testing.T) {
	anchor := time.Date(2026, 9, 10, 16, 45, 0, 0, time.UTC)
	tests := []struct {
		at, start, end string
	}{
		{"2026-09-10", "2026-09-10", "2026-10-10"},
		{"2026-10-09", "2026-09-10", "2026-10-10"},
		{"2026-10-10", "2026-10-10", "2026-11-09"},
		{"2026-12-09", "2026-12-09", "2027-01-08"},
	}
	for _, tc := range tests {
		at, _ := time.Parse("2006-01-02", tc.at)
		start, end := cycleWindow(anchor, at)
		if got := start.Format("2006-01-02"); got != tc.start {
			t.Fatalf("%s start expected %s got %s", tc.at, tc.start, got)
		}
		if got := end.Format("2006-01-02"); got != tc.end {
			t.Fatalf("%s end expected %s got %s", tc.at, tc.end, got)
		}
	}
}

func TestCycleBoundary(t *testing.T) {
	anchor, _ := time.Parse("2006-01-02", "2026-09-10")
	for _, date := range []string{"2026-10-10", "2026-11-09", "2027-01-08"} {
		at, _ := time.Parse("2006-01-02", date)
		if !isCycleBoundary(anchor, at) {
			t.Fatalf("expected boundary %s", date)
		}
	}
	for _, date := range []string{"2026-09-10", "2026-10-09", "2026-10-11"} {
		at, _ := time.Parse("2006-01-02", date)
		if isCycleBoundary(anchor, at) {
			t.Fatalf("unexpected boundary %s", date)
		}
	}
}

func TestDateOnlyUsesUTC(t *testing.T) {
	loc := time.FixedZone("test", -5*60*60)
	input := time.Date(2026, 11, 1, 23, 30, 0, 0, loc)
	got := dateOnly(input)
	if got.Location() != time.UTC {
		t.Fatalf("expected UTC")
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("expected UTC midnight got %v", got)
	}
}

func TestCancelAtPeriodEndBoundary(t *testing.T) {
	end, _ := time.Parse("2006-01-02", "2026-10-10")
	before, _ := time.Parse("2006-01-02", "2026-10-09")
	atEnd, _ := time.Parse("2006-01-02", "2026-10-10")
	if cancellationExpired(true, end, before) {
		t.Fatal("cancellation must not truncate the paid period")
	}
	if !cancellationExpired(true, end, atEnd) {
		t.Fatal("cancellation must stop renewal at the period boundary")
	}
	if cancellationExpired(false, end, atEnd) {
		t.Fatal("auto-renewing subscription must not expire at boundary")
	}
}
