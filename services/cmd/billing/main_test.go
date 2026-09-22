package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestModuleActivationDateUsesCatalogTimestamp(t *testing.T) {
	fallback, _ := time.Parse("2006-01-02", "2026-09-20")
	got := moduleActivationDate(map[string]any{"activated_at": "2026-09-12T15:04:05Z"}, fallback)
	if value := got.Format("2006-01-02"); value != "2026-09-12" {
		t.Fatalf("expected catalog activation date got %s", value)
	}
	got = moduleActivationDate(map[string]any{"activated_at": nil}, fallback)
	if value := got.Format("2006-01-02"); value != "2026-09-20" {
		t.Fatalf("expected fallback activation date got %s", value)
	}
}

func TestCommercialEvidenceKinds(t *testing.T) {
	for _, kind := range []string{"PAYMENT_EVIDENCE", "INVOICE", "RECEIPT", "CONTRACT", " receipt "} {
		if !isCommercialEvidenceKind(kind) {
			t.Fatalf("expected commercial evidence kind %q", kind)
		}
	}
	for _, kind := range []string{"OTHER", "NOTE", ""} {
		if isCommercialEvidenceKind(kind) {
			t.Fatalf("unexpected commercial evidence kind %q", kind)
		}
	}
}

func TestExpiredSubscriptionDisablesCatalogEntitlement(t *testing.T) {
	const token = "0123456789abcdefghijklmnop"
	var gotPath, gotMethod, gotToken, gotActor, gotStatus, gotReason string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Himate-Internal-Token")
		gotActor = r.Header.Get("X-Himate-User-ID")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotStatus, _ = body["status"].(string)
		gotReason, _ = body["reason"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"NOT_LICENSED"}`))
	}))
	defer server.Close()

	a := &app{
		catalogHost: strings.TrimPrefix(server.URL, "http://"),
		token: token,
		client: server.Client(),
	}
	if err := a.setCatalogModuleNotLicensed(context.Background(), "ptr_000002", "marketing_campaigns"); err != nil {
		t.Fatalf("catalog sync: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("expected PATCH got %s", gotMethod)
	}
	if gotPath != "/internal/v1/partners/ptr_000002/modules/marketing_campaigns" {
		t.Fatalf("unexpected path %s", gotPath)
	}
	if gotToken != token {
		t.Fatal("internal service credential missing")
	}
	if gotActor != "billing-cycle" {
		t.Fatalf("expected billing-cycle actor got %q", gotActor)
	}
	if gotStatus != "NOT_LICENSED" {
		t.Fatalf("expected NOT_LICENSED got %q", gotStatus)
	}
	if gotReason == "" {
		t.Fatal("history reason must not be empty")
	}
}

func TestSTART232CatalogPriceQuotesBatch(t *testing.T) {
	const token = "0123456789abcdefghijklmnop"
	var gotMethod, gotPath, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Himate-Internal-Token")
		var in struct {
			Items []map[string]string `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatalf("decode quote request: %v", err)
		}
		if len(in.Items) != 1 || in.Items[0]["partner_id"] != "ptr_232" || in.Items[0]["module_key"] != "ci.start232" || in.Items[0]["at"] != "2026-10-22" {
			t.Fatalf("unexpected quote request: %#v", in.Items)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"partner_id":"ptr_232","module_key":"ci.start232","at":"2026-10-22","price":91.25,"currency":"USD","included_in_base":false}],"count":1}`))
	}))
	defer server.Close()

	a := &app{
		catalogHost: strings.TrimPrefix(server.URL, "http://"),
		token: token,
		client: server.Client(),
	}
	quotes, err := a.catalogPriceQuotes(context.Background(), []map[string]string{{
		"partner_id":"ptr_232","module_key":"ci.start232","at":"2026-10-22",
	}})
	if err != nil {
		t.Fatalf("catalogPriceQuotes failed: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/internal/v1/module-price-quotes" {
		t.Fatalf("unexpected catalog quote request %s %s", gotMethod, gotPath)
	}
	if gotToken != token {
		t.Fatal("internal service credential missing")
	}
	quote, ok := quotes[quoteKey("ptr_232", "ci.start232")]
	if !ok || quote.Price != 91.25 || quote.Currency != "USD" || quote.Included {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestSTART223ModulePriceAtUsesPeriodStartContract(t *testing.T) {
	const token = "0123456789abcdefghijklmnop"
	var gotPath, gotAt, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAt = r.URL.Query().Get("at")
		gotToken = r.Header.Get("X-Himate-Internal-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"partner_id":"ptr_1","module_key":"ci.module","at":"2026-10-21","price":77.5,"currency":"USD","included_in_base":false,"source":"CATALOG_EFFECTIVE_PRICE_HISTORY"}`))
	}))
	defer server.Close()

	a := &app{
		catalogHost: strings.TrimPrefix(server.URL, "http://"),
		token: token,
		client: server.Client(),
	}
	at := time.Date(2026, 10, 21, 18, 30, 0, 0, time.UTC)
	price, included, currency, err := a.modulePriceAt(context.Background(), "ptr_1", "ci.module", at, 12, true, "USD")
	if err != nil {
		t.Fatalf("modulePriceAt failed: %v", err)
	}
	if gotPath != "/internal/v1/partners/ptr_1/modules/ci.module/price-at" {
		t.Fatalf("unexpected point-in-time price path: %s", gotPath)
	}
	if gotAt != "2026-10-21" {
		t.Fatalf("expected period-start date, got %s", gotAt)
	}
	if gotToken != token {
		t.Fatal("internal catalog credential missing")
	}
	if price != 77.5 || included || currency != "USD" {
		t.Fatalf("unexpected historical price contract: price=%v included=%v currency=%s", price, included, currency)
	}
}


func TestSTART233LifecycleMigrationContract(t *testing.T) {
	m := start233BillingLifecycleMigration()
	if m.Version != 7 {
		t.Fatalf("expected migration version 7 got %d", m.Version)
	}
	joined := strings.Join(m.Statements, "\n")
	for _, token := range []string{
		"lifecycle_state",
		"CANCEL_PENDING",
		"cancellation_requested_at",
		"cancellation_effective_at",
		"cancellation_requested_by",
		"cancellation_reason",
		"old_lifecycle_state",
		"new_lifecycle_state",
	} {
		if !strings.Contains(joined, token) {
			t.Fatalf("START-23.3 migration missing %q", token)
		}
	}
}

func TestSTART233CancellationBoundaryRemainsExclusive(t *testing.T) {
	end := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	if cancellationExpired(true, end, end.Add(-time.Second)) {
		t.Fatal("paid access must remain valid until the exclusive period boundary")
	}
	if !cancellationExpired(true, end, end) {
		t.Fatal("cancellation must become effective at the exact period boundary")
	}
}
