package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	centralStep4PartnersKey       = "partners_screen"
	centralStep4FinanceKey        = "finance_screen"
	centralStep4ImpactKey         = "impact_screen"
	centralStep4RefreshInterval   = 10 * time.Second
	centralStep4MaterializeBudget = 6 * time.Second
)

var centralStep4Refresh = struct {
	ch chan struct{}
}{
	ch: make(chan struct{}, 1),
}

func (a *app) requestCentralStep4Refresh() {
	select {
	case centralStep4Refresh.ch <- struct{}{}:
	default:
	}
}

func (a *app) runCentralStep4Materializer() {
	a.refreshCentralStep4Snapshots()
	ticker := time.NewTicker(centralStep4RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.refreshCentralStep4Snapshots()
		case <-centralStep4Refresh.ch:
			a.refreshCentralStep4Snapshots()
		}
	}
}

func (a *app) refreshCentralStep4Snapshots() {
	refreshes := []struct {
		key string
		fn  func()
	}{
		{centralStep4PartnersKey, a.refreshCentralStep4Partners},
		{centralStep4FinanceKey, a.refreshCentralStep4Finance},
		{centralStep4ImpactKey, a.refreshCentralStep4Impact},
	}
	for _, refresh := range refreshes {
		refresh := refresh
		if !centralStep3BeginRefresh(refresh.key) {
			continue
		}
		go func() {
			defer centralStep3EndRefresh(refresh.key)
			refresh.fn()
		}()
	}
}

func step4Map(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok {
		return centralStep3CopyMap(value)
	}
	return map[string]any{}
}

func step4Items(raw any) []map[string]any {
	return anyItems(raw)
}

func step4Unavailable(previous map[string]any, key string) any {
	if previous == nil {
		return nil
	}
	return previous[key]
}

// refreshCentralStep4Partners materializes the default Partners screen outside
// the request path. The first Central click can therefore serve a persistent,
// last-known-good read model instead of waiting for two sequential service
// fan-out waves (Partners -> Catalog/Billing/Health).
func (a *app) refreshCentralStep4Partners() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep4MaterializeBudget)
	defer cancel()

	var partners []map[string]any
	var categories, catalogPortfolio, billingPortfolio, healthPortfolio central10ItemsPage
	var partnerErr, categoriesErr, catalogErr, billingErr, healthErr error

	var first sync.WaitGroup
	first.Add(2)
	go func() {
		defer first.Done()
		partners, partnerErr = a.central10AllPartners(ctx)
	}()
	go func() {
		defer first.Done()
		categoriesErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partner-categories", &categories)
	}()
	first.Wait()

	// Never replace a last-known-good snapshot when the authoritative partner
	// list itself is unavailable.
	if partnerErr != nil {
		return
	}

	ids := make([]string, 0, len(partners))
	for _, item := range partners {
		if id := central10String(item["id"]); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		filter := url.QueryEscape(strings.Join(ids, ","))
		var enrich sync.WaitGroup
		enrich.Add(3)
		go func() {
			defer enrich.Done()
			catalogErr = a.internalGET(ctx, a.hosts["catalog"], "/internal/v1/portfolio?ids="+filter, &catalogPortfolio)
		}()
		go func() {
			defer enrich.Done()
			billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/portfolio?ids="+filter, &billingPortfolio)
		}()
		go func() {
			defer enrich.Done()
			healthErr = a.internalGET(ctx, a.hosts["health"], "/internal/v1/system-health/partner-snapshots?ids="+filter, &healthPortfolio)
		}()
		enrich.Wait()
	}

	catalogByID := map[string]map[string]any{}
	for _, item := range catalogPortfolio.Items {
		catalogByID[central10String(item["partner_id"])] = item
	}
	billingByID := map[string]map[string]any{}
	for _, item := range billingPortfolio.Items {
		billingByID[central10String(item["partner_id"])] = item
	}
	healthByID := map[string]map[string]any{}
	for _, item := range healthPortfolio.Items {
		healthByID[central10String(item["partner_id"])] = item
	}

	for _, partner := range partners {
		id := central10String(partner["id"])
		if cat := catalogByID[id]; cat != nil {
			partner["active_modules"] = central10Int(cat["active_modules"])
			partner["extra_module_fee"] = central10Float(cat["extra_module_fee"])
		}
		if bill := billingByID[id]; bill != nil {
			base := central10Float(bill["effective_base_fee"])
			extra := central10Float(partner["extra_module_fee"])
			partner["base_service_fee"] = base
			partner["service_value_30d"] = mathRound2(base + extra)
			if currency := central10String(bill["currency"]); currency != "" {
				partner["currency"] = currency
			}
		}
		if health := healthByID[id]; health != nil {
			if value := central10String(health["overall_status"]); value != "" {
				partner["system_health"] = value
			}
			if value := central10String(health["platform_version"]); value != "" {
				partner["platform_version"] = value
			}
			partner["connector_health"] = health["connector_health"]
			partner["environment_status"] = health["environment_status"]
			partner["provisioning_status"] = health["provisioning_status"]
		}
	}

	unavailable := []string{}
	if categoriesErr != nil { unavailable = append(unavailable, "partner_categories") }
	if catalogErr != nil { unavailable = append(unavailable, "catalog_portfolio") }
	if billingErr != nil { unavailable = append(unavailable, "billing_portfolio") }
	if healthErr != nil { unavailable = append(unavailable, "health_portfolio") }
	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}

	lifecycleCounts := map[string]int{}
	referenceCount := 0
	for _, partner := range partners {
		lifecycle := strings.ToUpper(central10String(partner["lifecycle"]))
		if lifecycle != "" {
			lifecycleCounts[lifecycle]++
		}
		if partner["reference_partner"] == true {
			referenceCount++
		}
	}
	recordTotal := len(partners)
	defaultCount := recordTotal
	if defaultCount > 24 {
		defaultCount = 24
	}

	payload := map[string]any{
		// Keep the complete enriched portfolio in the private materialized
		// snapshot. HTTP pagination/search is applied when the snapshot is read.
		"items":          partners,
		"categories_raw": categories.Items,
		"pagination": map[string]any{
			"count": defaultCount, "total": recordTotal, "limit": 24,
			"offset": 0, "has_more": recordTotal > defaultCount,
		},
		"kpis": map[string]any{
			"partner_records":    recordTotal,
			"live_partners":      lifecycleCounts["LIVE"],
			"prospects":          lifecycleCounts["PROSPECT"],
			"reference_partners": referenceCount,
			"lifecycle_counts":   lifecycleCounts,
		},
		"status":      status,
		"unavailable": unavailable,
	}

	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep4PartnersKey, payload)
}

func (a *app) refreshCentralStep4Finance() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep4MaterializeBudget)
	defer cancel()

	var profile, overview map[string]any
	var invoices central10ItemsPage
	var partners []map[string]any
	var profileErr, overviewErr, invoicesErr, partnersErr error

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		profileErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/profile", &profile)
	}()
	go func() {
		defer wg.Done()
		overviewErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/finance/overview", &overview)
	}()
	go func() {
		defer wg.Done()
		invoicesErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/invoices", &invoices)
	}()
	go func() {
		defer wg.Done()
		partners, partnersErr = a.central10AllPartners(ctx)
	}()
	wg.Wait()

	previous, _, _ := centralStep3SnapshotGet(centralStep4FinanceKey)
	successful := 0
	unavailable := []string{}

	if profileErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "billing_profile")
		profile = step4Map(step4Unavailable(previous, "profile"))
	}
	if overviewErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "finance_overview")
		overview = step4Map(step4Unavailable(previous, "overview"))
	}
	if invoicesErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "invoices")
		invoices.Items = step4Items(step4Unavailable(previous, "invoices"))
	}
	if partnersErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "partners")
		partners = step4Items(step4Unavailable(previous, "partners"))
	}
	status := "healthy"
	if successful == 0 && previous == nil {
		// Persist an explicit unavailable snapshot instead of leaving the screen
		// in a permanent warming state when every dependency is down or
		// misconfigured. The frontend can then render a deterministic error/
		// empty state and still offer a manual refresh.
		status = "unavailable"
	} else if len(unavailable) > 0 {
		status = "partial"
	}
	payload := map[string]any{
		"profile":     profile,
		"overview":    overview,
		"invoices":    invoices.Items,
		"partners":    partners,
		"status":      status,
		"unavailable": unavailable,
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep4FinanceKey, payload)
}

func (a *app) centralStep4AllEvidence(ctx context.Context) ([]map[string]any, error) {
	const pageSize = 100
	var first central10ItemsPage
	if err := a.internalGET(ctx, a.hosts["evidence"], "/api/v1/evidence?limit=100&offset=0", &first); err != nil {
		return nil, err
	}
	if !first.HasMore || first.Total <= len(first.Items) {
		return first.Items, nil
	}

	pageCount := (first.Total + pageSize - 1) / pageSize
	pages := make([][]map[string]any, pageCount)
	pages[0] = first.Items
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for pageIndex := 1; pageIndex < pageCount; pageIndex++ {
		pageIndex := pageIndex
		offset := pageIndex * pageSize
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errMu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				errMu.Unlock()
				return
			}
			var page central10ItemsPage
			path := fmt.Sprintf("/api/v1/evidence?limit=%d&offset=%d", pageSize, offset)
			if err := a.internalGET(ctx, a.hosts["evidence"], path, &page); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			pages[pageIndex] = page.Items
		}()
	}
	wg.Wait()

	items := make([]map[string]any, 0, first.Total)
	for _, page := range pages {
		items = append(items, page...)
	}
	if firstErr != nil {
		return items, firstErr
	}
	return items, nil
}

func (a *app) refreshCentralStep4Impact() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep4MaterializeBudget)
	defer cancel()

	var definitions, summary, reports central10ItemsPage
	var evidence []map[string]any
	var analytics map[string]any
	var definitionsErr, summaryErr, evidenceErr, reportsErr, analyticsErr error

	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		definitionsErr = a.internalGET(ctx, a.hosts["impact"], "/api/v1/impact/definitions", &definitions)
	}()
	go func() {
		defer wg.Done()
		summaryErr = a.internalGET(ctx, a.hosts["impact"], "/api/v1/impact/summary", &summary)
	}()
	go func() {
		defer wg.Done()
		evidence, evidenceErr = a.centralStep4AllEvidence(ctx)
	}()
	go func() {
		defer wg.Done()
		reportsErr = a.internalGET(ctx, a.hosts["reports"], "/api/v1/reports", &reports)
	}()
	go func() {
		defer wg.Done()
		analyticsErr = a.internalGET(ctx, a.hosts["impact"], "/internal/v1/impact/dashboard?year="+strconv.Itoa(time.Now().UTC().Year()), &analytics)
	}()
	wg.Wait()

	previous, _, _ := centralStep3SnapshotGet(centralStep4ImpactKey)
	successful := 0
	unavailable := []string{}

	if definitionsErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "impact_definitions")
		definitions.Items = step4Items(step4Unavailable(previous, "definitions"))
	}
	if summaryErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "impact_summary")
		summary.Items = step4Items(step4Unavailable(previous, "summary"))
	}
	if evidenceErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "evidence")
		evidence = step4Items(step4Unavailable(previous, "evidence"))
	}
	if reportsErr == nil {
		successful++
	} else {
		unavailable = append(unavailable, "reports")
		reports.Items = step4Items(step4Unavailable(previous, "reports"))
	}
	if analyticsErr == nil && analytics != nil {
		successful++
		analytics = central10NormalizeDashboardImpact(analytics)
	} else {
		unavailable = append(unavailable, "impact_analytics")
		analytics = step4Map(step4Unavailable(previous, "analytics"))
	}
	// A first-run total upstream outage must still materialize a degraded
	// snapshot. Otherwise /api/v1/central/impact can remain in WARMING forever
	// even though the UI is capable of rendering explicit empty/unavailable
	// states. Previous data is retained per-source above when it exists.
	status := "healthy"
	if successful == 0 {
		status = "unavailable"
	} else if len(unavailable) > 0 {
		status = "partial"
	}
	payload := map[string]any{
		"definitions": definitions.Items,
		"summary":     summary.Items,
		"evidence":    evidence,
		"reports":     reports.Items,
		"analytics":   analytics,
		"status":      status,
		"unavailable": unavailable,
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep4ImpactKey, payload)
}

func centralStep4Meta(
	started time.Time,
	snapshotKey string,
	updatedAt time.Time,
	status string,
	unavailable []string,
) map[string]any {
	meta := central10Meta(started, status, unavailable)
	meta["delivery"] = "MATERIALIZED_HOT_SNAPSHOT"
	meta["snapshot_key"] = snapshotKey
	meta["refresh_interval_ms"] = centralStep4RefreshInterval.Milliseconds()
	if !updatedAt.IsZero() {
		meta["snapshot_updated_at"] = updatedAt.UTC()
		meta["snapshot_age_ms"] = time.Since(updatedAt).Milliseconds()
	}
	return meta
}
