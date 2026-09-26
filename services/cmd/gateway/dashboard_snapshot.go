package main

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const (
	dashboardSnapshotFreshTTL       = 30 * time.Second
	dashboardSnapshotRefreshInterval = 10 * time.Second
	dashboardMaterializeBudget       = 4 * time.Second
)

func central10DashboardSnapshotMigration() common.Migration {
	return common.Migration{
		Version: 17,
		Name:    "central-10-1-dashboard-materialized-snapshot",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.dashboard_snapshots(
				year INTEGER PRIMARY KEY,
				payload JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_dashboard_snapshots_updated_idx ON identity.dashboard_snapshots(updated_at DESC)`,
		},
	}
}

func dashboardCopyBlock(raw any) map[string]any {
	if block, ok := raw.(map[string]any); ok {
		return copyDashboardPayload(block)
	}
	return map[string]any{}
}

func dashboardUnavailableBlock() map[string]any {
	return map[string]any{"available": false, "status": "unavailable"}
}

func dashboardStaleBlock(raw any) map[string]any {
	block := dashboardCopyBlock(raw)
	if len(block) == 0 {
		return dashboardUnavailableBlock()
	}
	block["available"] = true
	block["status"] = "stale"
	return block
}

func dashboardFreshBlock(raw map[string]any) map[string]any {
	block := copyDashboardPayload(raw)
	block["available"] = true
	if _, ok := block["status"]; !ok {
		block["status"] = "healthy"
	}
	return block
}

func (a *app) loadDashboardSnapshot(year int) (map[string]any, time.Time, error) {
	var raw []byte
	var updated time.Time
	err := a.db.QueryRow(
		`SELECT payload,updated_at FROM identity.dashboard_snapshots WHERE year=$1`,
		year,
	).Scan(&raw, &updated)
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, time.Time{}, err
	}
	return payload, updated.UTC(), nil
}

func (a *app) materializeDashboardActivity(ctx context.Context) ([]map[string]any, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT id,actor_name,action,resource,partner_id,outcome,created_at
		FROM identity.audit_events
		WHERE outcome='SUCCESS'
		ORDER BY created_at DESC,id DESC
		LIMIT 120`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]map[string]any, 0, 120)
	for rows.Next() {
		var id int64
		var actorName, action, resource, partnerID, outcome string
		var created time.Time
		if err := rows.Scan(&id, &actorName, &action, &resource, &partnerID, &outcome, &created); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "actor_name": actorName, "action": action, "resource": resource,
			"partner_id": partnerID, "outcome": outcome, "created_at": created.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (a *app) persistDashboardSnapshot(ctx context.Context, year int, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(
		ctx,
		`INSERT INTO identity.dashboard_snapshots(year,payload,updated_at)
		 VALUES($1,$2::jsonb,NOW())
		 ON CONFLICT(year) DO UPDATE SET payload=EXCLUDED.payload,updated_at=NOW()`,
		year,
		string(raw),
	)
	return err
}

func (a *app) bootstrapDashboardSnapshot() {
	year := time.Now().UTC().Year()
	payload, updated, err := a.loadDashboardSnapshot(year)
	if err != nil {
		return
	}
	a.dashboardMu.Lock()
	a.dashboardPayload = payload
	a.dashboardUpdatedAt = updated
	a.dashboardExpires = updated.Add(dashboardSnapshotFreshTTL)
	a.dashboardMu.Unlock()
}

func (a *app) requestDashboardRefresh() {
	if a.dashboardRefreshCh == nil {
		return
	}
	select {
	case a.dashboardRefreshCh <- struct{}{}:
	default:
	}
}

func (a *app) runDashboardMaterializer() {
	a.refreshDashboardSnapshot(time.Now().UTC().Year())
	ticker := time.NewTicker(dashboardSnapshotRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.refreshDashboardSnapshot(time.Now().UTC().Year())
		case <-a.dashboardRefreshCh:
			a.refreshDashboardSnapshot(time.Now().UTC().Year())
		}
	}
}

func (a *app) refreshDashboardSnapshot(year int) {
	a.dashboardRefreshMu.Lock()
	defer a.dashboardRefreshMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), dashboardMaterializeBudget)
	defer cancel()

	payload, changed := a.materializeDashboardSnapshot(ctx, year)
	if payload == nil {
		return
	}

	now := time.Now().UTC()
	a.dashboardMu.Lock()
	a.dashboardPayload = payload
	a.dashboardUpdatedAt = now
	a.dashboardExpires = now.Add(dashboardSnapshotFreshTTL)
	a.dashboardMu.Unlock()

	if changed {
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = a.persistDashboardSnapshot(persistCtx, year, payload)
		persistCancel()
	}
}

func (a *app) materializeDashboardSnapshot(ctx context.Context, year int) (map[string]any, bool) {
	a.dashboardMu.RLock()
	previous := copyDashboardPayload(a.dashboardPayload)
	a.dashboardMu.RUnlock()
	if previousYear, ok := previous["year"].(float64); ok && int(previousYear) != year {
		previous = nil
	}
	if previousYear, ok := previous["year"].(int); ok && previousYear != year {
		previous = nil
	}
	if len(previous) == 0 {
		if stored, _, err := a.loadDashboardSnapshot(year); err == nil {
			previous = stored
		}
	}

	type partnerResult struct {
		Items           []map[string]any `json:"items"`
		Total           int              `json:"total"`
		LifecycleCounts map[string]int   `json:"lifecycle_counts"`
	}
	type moduleResult struct {
		Items []map[string]any `json:"items"`
		Count int              `json:"count"`
	}

	var partners partnerResult
	var modules moduleResult
	var billing map[string]any
	var impact map[string]any
	var activity []map[string]any
	var partnerErr, moduleErr, billingErr, impactErr, activityErr error
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		partnerErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partners?limit=1&offset=0&include_archived=true", &partners)
	}()
	go func() {
		defer wg.Done()
		moduleErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules)
	}()
	go func() {
		defer wg.Done()
		billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/analytics/dashboard?year="+strconv.Itoa(year), &billing)
	}()
	go func() {
		defer wg.Done()
		impactErr = a.internalGET(ctx, a.hosts["impact"], "/internal/v1/impact/dashboard?year="+strconv.Itoa(year), &impact)
	}()
	go func() {
		defer wg.Done()
		activity, activityErr = a.materializeDashboardActivity(ctx)
	}()
	wg.Wait()

	unavailable := make([]string, 0, 4)
	successful := 0

	partnerBlock := dashboardStaleBlock(previous["partners"])
	if partnerErr == nil {
		successful++
		partnerBlock = dashboardFreshBlock(map[string]any{
			"total":            partners.Total,
			"live":             partners.LifecycleCounts["LIVE"],
			"lifecycle_counts": partners.LifecycleCounts,
		})
	} else {
		unavailable = append(unavailable, "partners")
	}

	moduleBlock := dashboardStaleBlock(previous["modules"])
	if moduleErr == nil {
		successful++
		moduleBlock = dashboardFreshBlock(map[string]any{"catalog_total": modules.Count})
	} else {
		unavailable = append(unavailable, "modules")
	}

	billingBlock := dashboardStaleBlock(previous["billing"])
	if billingErr == nil && billing != nil {
		successful++
		billingBlock = dashboardFreshBlock(billing)
	} else {
		unavailable = append(unavailable, "billing")
	}

	impactBlock := dashboardStaleBlock(previous["impact"])
	if impactErr == nil && impact != nil {
		successful++
		impact = central10NormalizeDashboardImpact(impact)
		impactBlock = dashboardFreshBlock(impact)
	} else {
		unavailable = append(unavailable, "impact")
	}

	activityBlock := map[string]any{
		"items": activity,
		"count": len(activity),
		"source": "IDENTITY_APPEND_ONLY_AUDIT",
		"status": "healthy",
	}
	if activityErr != nil {
		activityBlock = map[string]any{
			"items": []any{},
			"count": 0,
			"source": "IDENTITY_APPEND_ONLY_AUDIT",
			"status": "degraded",
		}
		unavailable = append(unavailable, "activity")
	}

	sort.Strings(unavailable)
	status := "healthy"
	if len(unavailable) > 0 {
		status = "degraded"
	}
	generatedAt := time.Now().UTC()
	payload := map[string]any{
		"year":     year,
		"partners": partnerBlock,
		"modules":  moduleBlock,
		"billing":  billingBlock,
		"impact":   impactBlock,
		"activity": activityBlock,
		"system": map[string]any{
			"status":       status,
			"environment":  a.env,
			"version":      a.version,
			"architecture": "materialized-dashboard-snapshot",
		},
		"meta": map[string]any{
			"architecture":    "MATERIALIZED_DASHBOARD_SNAPSHOT",
			"status":          status,
			"unavailable":     unavailable,
			"generated_at":    generatedAt,
			"refresh_interval_ms": dashboardSnapshotRefreshInterval.Milliseconds(),
		},
	}
	return payload, successful > 0
}

func (a *app) dashboardSnapshotForRead(year int) (map[string]any, time.Time, bool) {
	currentYear := time.Now().UTC().Year()
	if year == currentYear {
		a.dashboardMu.RLock()
		payload := copyDashboardPayload(a.dashboardPayload)
		updated := a.dashboardUpdatedAt
		expires := a.dashboardExpires
		a.dashboardMu.RUnlock()
		if len(payload) > 0 {
			return payload, updated, time.Now().After(expires)
		}
		// bootstrapDashboardSnapshot already attempted the persisted snapshot before
		// the HTTP server started. A current-year request must therefore never fall
		// back to synchronous database I/O: return the warming contract immediately
		// while the background materializer builds the first hot snapshot.
		return nil, time.Time{}, true
	}
	payload, updated, err := a.loadDashboardSnapshot(year)
	if err != nil {
		return nil, time.Time{}, true
	}
	return payload, updated, time.Since(updated) > dashboardSnapshotFreshTTL
}

func dashboardWarmingSnapshot(year int, env, version string) map[string]any {
	return map[string]any{
		"year":     year,
		"partners": dashboardUnavailableBlock(),
		"modules":  dashboardUnavailableBlock(),
		"billing":  dashboardUnavailableBlock(),
		"impact":   dashboardUnavailableBlock(),
		"activity": map[string]any{"items": []any{}, "count": 0, "source": "IDENTITY_APPEND_ONLY_AUDIT", "status": "warming"},
		"system": map[string]any{
			"status":       "warming",
			"environment":  env,
			"version":      version,
			"architecture": "materialized-dashboard-snapshot",
		},
		"meta": map[string]any{
			"architecture": "MATERIALIZED_DASHBOARD_SNAPSHOT",
			"status":       "warming",
			"unavailable":  []string{"activity", "billing", "impact", "modules", "partners"},
			"generated_at": time.Now().UTC(),
		},
	}
}
