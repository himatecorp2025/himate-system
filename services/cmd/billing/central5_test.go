package main

import "testing"

func TestCentral5StandardPackageContract(t *testing.T) {
	for key, want := range map[string]int{"STARTER": 10, "BUSINESS": 20, "FLEX": 0} {
		got, ok := standardPlanLimit(key)
		if !ok || got != want {
			t.Fatalf("%s limit=%d ok=%v want=%d", key, got, ok, want)
		}
	}
}

func TestCentral5TaxCalculation(t *testing.T) {
	policy := billingTaxPolicy{RatePercent: 20, Label: "VAT", Jurisdiction: "GB"}
	net, tax, gross := applyBillingTax(1490, policy)
	if net != 1490 || tax != 298 || gross != 1788 {
		t.Fatalf("unexpected tax calculation net=%v tax=%v gross=%v", net, tax, gross)
	}
	_, zeroTax, zeroGross := applyBillingTax(990, billingTaxPolicy{})
	if zeroTax != 0 || zeroGross != 990 {
		t.Fatalf("zero-rate VAT must preserve net price: tax=%v gross=%v", zeroTax, zeroGross)
	}
}
