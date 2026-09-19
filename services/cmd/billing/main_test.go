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
