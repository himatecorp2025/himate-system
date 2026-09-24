package main

import "testing"

func TestCalculateItemsUsesMinorUnitsAndTaxBPS(t *testing.T) {
	items, subtotal, tax, total, err := calculateItems([]invoiceItemInput{{
		Description: "Piano tuning", QuantityMilli: 1500, UnitPriceMinor: 20000, DiscountMinor: 2500, TaxRateBPS: 887,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if subtotal != 27500 {
		t.Fatalf("subtotal=%d want 27500", subtotal)
	}
	if tax != 2439 {
		t.Fatalf("tax=%d want 2439", tax)
	}
	if total != 29939 {
		t.Fatalf("total=%d want 29939", total)
	}
}

func TestAutomatedSourceIsBoundToServiceIdentity(t *testing.T) {
	if err := validateSourceForService("workshop", sourceWorkflow); err != nil {
		t.Fatalf("workshop should produce workflow invoices: %v", err)
	}
	if err := validateSourceForService("scheduler", sourceSchedule); err != nil {
		t.Fatalf("scheduler should produce schedule invoices: %v", err)
	}
	if err := validateSourceForService("scheduler", sourceWorkflow); err == nil {
		t.Fatal("scheduler must not impersonate workshop invoice source")
	}
	if err := validateSourceForService("workshop", sourceSchedule); err == nil {
		t.Fatal("workshop must not impersonate scheduler invoice source")
	}
}

func TestNormalizeIssuerRequiresTenantBrandAndUSBillingIdentity(t *testing.T) {
	_, err := normalizeIssuer("ptr_1", map[string]any{
		"legal_name": "Example LLC", "country": "United States", "city": "New York",
		"state_region": "NY", "postal_code": "10001", "address_line1": "1 Example Ave",
	})
	if err == nil {
		t.Fatal("invoice-ready issuer must require logo_url")
	}
	issuer, err := normalizeIssuer("ptr_1", map[string]any{
		"legal_name": "Example LLC", "display_name": "Example", "country": "United States", "city": "New York",
		"state_region": "NY", "postal_code": "10001", "address_line1": "1 Example Ave",
		"logo_url": "https://cdn.example/logo.png", "tax_id": "12-3456789",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issuer.PartnerID != "ptr_1" || issuer.LegalName != "Example LLC" {
		t.Fatalf("unexpected issuer snapshot: %+v", issuer)
	}
}

func TestManualCustomerMayBeDraftedBeforeFullBillingAddress(t *testing.T) {
	customer, err := normalizeCustomer(customerSnapshot{DisplayName: "Jane Doe"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if customer.DisplayName != "Jane Doe" {
		t.Fatalf("unexpected customer: %+v", customer)
	}
	if _, err := normalizeCustomer(customer, true); err == nil {
		t.Fatal("finalization must require a billing address")
	}
}

func TestInvoicePrefixValidation(t *testing.T) {
	if got, err := normalizeInvoicePrefix(" kh "); err != nil || got != "KH" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := normalizeInvoicePrefix("bad prefix"); err == nil {
		t.Fatal("prefix containing spaces must be rejected")
	}
}
