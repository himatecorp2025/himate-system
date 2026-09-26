package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const (
	centralStep3RegistryKey      = "modules_registry"
	centralStep3CommercialKey    = "modules_commercial"
	centralStep3PlansKey         = "billing_plans"
	centralStep3AnalyticsKey     = "package_analytics"
	centralStep3RefreshInterval  = 10 * time.Second
	centralStep3MaterializeBudget = 5 * time.Second
)

type centralStep3SnapshotEntry struct {
	payload   map[string]any
	updatedAt time.Time
}

var centralStep3Snapshots = struct {
	sync.RWMutex
	items      map[string]centralStep3SnapshotEntry
	refreshMu  sync.Mutex
	refreshing map[string]bool
	refreshCh  chan struct{}
}{
	items:      map[string]centralStep3SnapshotEntry{},
	refreshing: map[string]bool{},
	refreshCh:  make(chan struct{}, 1),
}

func central10Step3SnapshotMigration() common.Migration {
	return common.Migration{
		Version: 18,
		Name:    "central-10-1-modules-packages-screen-snapshots",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.central_screen_snapshots(
				snapshot_key TEXT PRIMARY KEY,
				payload JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_central_screen_snapshots_updated_idx
			  ON identity.central_screen_snapshots(updated_at DESC)`,
		},
	}
}

func centralStep3CopyMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func (a *app) bootstrapCentralStep3Snapshots() {
	rows, err := a.db.Query(`SELECT snapshot_key,payload,updated_at FROM identity.central_screen_snapshots`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw []byte
		var updated time.Time
		if err := rows.Scan(&key, &raw, &updated); err != nil {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated.UTC(),
		}
		centralStep3Snapshots.Unlock()
	}
}

func centralStep3SnapshotGet(key string) (map[string]any, time.Time, bool) {
	centralStep3Snapshots.RLock()
	entry, ok := centralStep3Snapshots.items[key]
	centralStep3Snapshots.RUnlock()
	if !ok || entry.payload == nil {
		return nil, time.Time{}, false
	}
	return centralStep3CopyMap(entry.payload), entry.updatedAt, true
}

func (a *app) centralStep3Store(ctx context.Context, key string, payload map[string]any) {
	if payload == nil {
		return
	}
	now := time.Now().UTC()
	copyPayload := centralStep3CopyMap(payload)
	centralStep3Snapshots.Lock()
	centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
		payload: copyPayload, updatedAt: now,
	}
	centralStep3Snapshots.Unlock()

	raw, err := json.Marshal(copyPayload)
	if err != nil {
		return
	}
	_, _ = a.db.ExecContext(
		ctx,
		`INSERT INTO identity.central_screen_snapshots(snapshot_key,payload,updated_at)
		 VALUES($1,$2::jsonb,NOW())
		 ON CONFLICT(snapshot_key) DO UPDATE SET payload=EXCLUDED.payload,updated_at=NOW()`,
		key,
		string(raw),
	)
}

func (a *app) requestCentralStep3Refresh() {
	select {
	case centralStep3Snapshots.refreshCh <- struct{}{}:
	default:
	}
}

func centralStep3BeginRefresh(key string) bool {
	centralStep3Snapshots.refreshMu.Lock()
	defer centralStep3Snapshots.refreshMu.Unlock()
	if centralStep3Snapshots.refreshing[key] {
		return false
	}
	centralStep3Snapshots.refreshing[key] = true
	return true
}

func centralStep3EndRefresh(key string) {
	centralStep3Snapshots.refreshMu.Lock()
	delete(centralStep3Snapshots.refreshing, key)
	centralStep3Snapshots.refreshMu.Unlock()
}

func (a *app) runCentralStep3Materializer() {
	a.refreshCentralStep3Snapshots()
	ticker := time.NewTicker(centralStep3RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.refreshCentralStep3Snapshots()
		case <-centralStep3Snapshots.refreshCh:
			a.refreshCentralStep3Snapshots()
		}
	}
}

func (a *app) refreshCentralStep3Snapshots() {
	refreshes := []struct {
		key string
		fn  func()
	}{
		{centralStep3RegistryKey, a.refreshCentralStep3Registry},
		{centralStep3PlansKey, a.refreshCentralStep3Plans},
		{centralStep3AnalyticsKey, a.refreshCentralStep3Analytics},
		{centralStep3CommercialKey, a.refreshCentralStep3Commercial},
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

func (a *app) refreshCentralStep3Registry() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	type result struct {
		kind  string
		page  central10ItemsPage
		err   error
	}
	results := make(chan result, 2)
	go func() {
		var page central10ItemsPage
		err := a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &page)
		results <- result{kind: "modules", page: page, err: err}
	}()
	go func() {
		var page central10ItemsPage
		err := a.internalGET(ctx, a.hosts["catalog"], "/api/v1/module-groups", &page)
		results <- result{kind: "groups", page: page, err: err}
	}()

	var modules, groups []map[string]any
	var failed bool
	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			failed = true
			continue
		}
		if res.kind == "modules" {
			modules = res.page.Items
		} else {
			groups = res.page.Items
		}
	}
	if failed {
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3RegistryKey, map[string]any{
		"modules": modules,
		"groups":  groups,
	})
}

func (a *app) refreshCentralStep3Plans() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()
	var page central10ItemsPage
	if err := a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/plans", &page); err != nil {
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3PlansKey, map[string]any{"plans": page.Items})
}

func (a *app) refreshCentralStep3Analytics() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()
	var analytics map[string]any
	if err := a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/packages/analytics", &analytics); err != nil {
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3AnalyticsKey, map[string]any{"analytics": analytics})
}

func (a *app) refreshCentralStep3Commercial() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	partners, partnerErr := a.central10AllPartners(ctx)
	if partnerErr != nil {
		return
	}
	partnerIDs := make([]string, 0, len(partners))
	for _, partner := range partners {
		if id := central10String(partner["id"]); id != "" {
			partnerIDs = append(partnerIDs, id)
		}
	}
	sort.Strings(partnerIDs)

	matrixItems, subscriptionItems, matrixErr, subscriptionsErr := a.central10CommercialSources(ctx, partnerIDs, true)
	previous, _, _ := centralStep3SnapshotGet(centralStep3CommercialKey)
	if matrixErr != nil {
		matrixItems = anyItems(previous["matrix_items"])
	}
	if subscriptionsErr != nil {
		subscriptionItems = anyItems(previous["subscription_items"])
	}
	payload := map[string]any{
		"partners":            partners,
		"matrix_items":        matrixItems,
		"subscription_items":  subscriptionItems,
		"matrix_available":    matrixErr == nil || len(matrixItems) > 0,
		"subscriptions_available": subscriptionsErr == nil || len(subscriptionItems) > 0,
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3CommercialKey, payload)
}

func centralStep3Meta(
	started time.Time,
	snapshotKey string,
	updatedAt time.Time,
	status string,
	unavailable []string,
) map[string]any {
	meta := central10Meta(started, status, unavailable)
	meta["delivery"] = "MATERIALIZED_HOT_SNAPSHOT"
	meta["snapshot_key"] = snapshotKey
	meta["refresh_interval_ms"] = centralStep3RefreshInterval.Milliseconds()
	if !updatedAt.IsZero() {
		meta["snapshot_updated_at"] = updatedAt.UTC()
		meta["snapshot_age_ms"] = time.Since(updatedAt).Milliseconds()
	}
	return meta
}
