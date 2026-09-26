package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
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
	if successful == 0 && previous == nil {
		return
	}

	status := "healthy"
	if len(unavailable) > 0 {
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
	var definitionsErr, summaryErr, evidenceErr, reportsErr error

	var wg sync.WaitGroup
	wg.Add(4)
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
	if successful == 0 && previous == nil {
		return
	}

	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	payload := map[string]any{
		"definitions": definitions.Items,
		"summary":     summary.Items,
		"evidence":    evidence,
		"reports":     reports.Items,
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
