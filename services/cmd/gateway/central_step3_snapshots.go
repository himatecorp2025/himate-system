package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

// centralSnapshotValid is the hard Last-Known-Good gate for every Central
// screen snapshot stored in identity.central_screen_snapshots. A degraded
// refresh is observability data, not a new screen state: it must never replace
// a complete model that the browser can already render.
func centralSnapshotValid(key string, payload map[string]any) bool {
	if payload == nil || !strings.EqualFold(central10String(payload["status"]), "healthy") {
		return false
	}
	if raw, exists := payload["unavailable"]; exists && len(central10Step4Unavailable(raw)) > 0 {
		return false
	}
	required := []string{}
	switch key {
	case centralStep3RegistryKey:
		required = []string{"modules", "groups", "trend", "module_details"}
	case centralStep3PlansKey:
		required = []string{"plans", "modules"}
	case centralStep3AnalyticsKey:
		required = []string{"analytics", "modules"}
	case centralStep3CommercialKey:
		required = []string{"partners", "matrix_items", "subscription_items", "modules", "plans", "matrix_available", "subscriptions_available"}
		if payload["matrix_available"] != true || payload["subscriptions_available"] != true {
			return false
		}
	case centralStep4PartnersKey:
		required = []string{"items", "categories_raw", "pagination", "kpis"}
	case centralStep4FinanceKey:
		required = []string{"profile", "overview", "invoices", "partners"}
	case centralStep4ImpactKey:
		required = []string{"definitions", "summary", "evidence", "evidence_integrity", "reports", "analytics"}
	case centralStep4AdministrationKey:
		required = []string{"company", "items", "kpis", "admin_roles", "admin_users", "admin_secrets", "audit_events", "company_documents", "invoice_register", "backup_api"}
	case centralStep4SystemKey:
		required = []string{
			"health", "provisioning", "environments", "events", "backups", "kpis",
			"health_api", "provisioning_api", "environments_api", "backups_api",
			"backup_restore_points", "backup_restore_tests_api", "backup_restore_jobs_api",
		}
	case centralStep4WebsiteKey:
		required = []string{"pages", "media", "environments", "kpis", "seo", "seo_audit", "contact_inquiries", "cms_design", "cms_page_details", "cms_page_versions", "cms_page_audits"}
	case centralStep4ConnectionsKey:
		required = []string{"items", "kpis", "start22_mapping", "start22_summary"}
	case centralStep4ComplianceKey:
		required = []string{"items", "details"}
	case centralStep4GlobalSearchKey:
		required = []string{"partners", "modules", "contact_inquiries", "cms_pages", "admin_users", "audit_events"}
	default:
		if strings.HasPrefix(key, centralPartnerWorkspacePrefix) && centralPartnerWorkspaceID(key) != "" {
			return partnerWorkspaceSnapshotValid(payload)
		}
		return false
	}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return false
		}
	}
	return true
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
		if strings.HasPrefix(key, centralPartnerWorkspacePrefix) {
			// CENTRAL-21 migrates tenant workspaces to the dedicated
			// identity.partner_workspace_snapshots table.
			continue
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		if !centralSnapshotValid(key, payload) {
			if a.log != nil {
				a.log.Warn("central snapshot bootstrap rejected non-LKG payload", "snapshot_key", key)
			}
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

func (a *app) warmMissingCentralSnapshots() {
	step3 := []struct {
		key string
		fn  func()
	}{
		{centralStep3RegistryKey, a.refreshCentralStep3Registry},
		{centralStep3PlansKey, a.refreshCentralStep3Plans},
		{centralStep3AnalyticsKey, a.refreshCentralStep3Analytics},
		{centralStep3CommercialKey, a.refreshCentralStep3Commercial},
	}
	step4 := []struct {
		key string
		fn  func()
	}{
		{centralStep4PartnersKey, a.refreshCentralStep4Partners},
		{centralStep4FinanceKey, a.refreshCentralStep4Finance},
		{centralStep4ImpactKey, a.refreshCentralStep4Impact},
		{centralStep4AdministrationKey, a.refreshCentralStep4Administration},
		{centralStep4SystemKey, a.refreshCentralStep4System},
		{centralStep4WebsiteKey, a.refreshCentralStep4Website},
		{centralStep4ConnectionsKey, a.refreshCentralStep4Connections},
		{centralStep4ComplianceKey, a.refreshCentralStep4Compliance},
		{centralStep4GlobalSearchKey, a.refreshCentralStep4GlobalSearch},
	}

	warm := func(jobs []struct {
		key string
		fn  func()
	}) {
		var wg sync.WaitGroup
		for _, job := range jobs {
			if _, _, ok := centralStep3SnapshotGet(job.key); ok {
				continue
			}
			wg.Add(1)
			go func(fn func()) {
				defer wg.Done()
				fn()
			}(job.fn)
		}
		wg.Wait()
	}

	// A legacy CENTRAL-20 degraded row is intentionally ignored by bootstrap.
	// Give dependent services a few startup windows to publish one complete LKG
	// model before normal traffic reaches the Central shell.
	for attempt := 1; attempt <= 3; attempt++ {
		warm(step3)
		warm(step4)
		missing := []string{}
		for _, job := range step3 {
			if _, _, ok := centralStep3SnapshotGet(job.key); !ok {
				missing = append(missing, job.key)
			}
		}
		for _, job := range step4 {
			if _, _, ok := centralStep3SnapshotGet(job.key); !ok {
				missing = append(missing, job.key)
			}
		}
		if len(missing) == 0 {
			return
		}
		if a.log != nil {
			a.log.Warn("central startup read-model warmup incomplete", "attempt", attempt, "missing", missing)
		}
		if attempt < 3 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (a *app) centralStep3Store(ctx context.Context, key string, payload map[string]any) {
	if strings.HasPrefix(key, centralPartnerWorkspacePrefix) {
		a.persistPartnerWorkspaceSnapshot(ctx, centralPartnerWorkspaceID(key), payload)
		return
	}
	if !centralSnapshotValid(key, payload) {
		if a.log != nil {
			a.log.Warn(
				"central read-model refresh rejected; retaining last-known-good snapshot",
				"snapshot_key", key,
				"status", central10String(payload["status"]),
				"unavailable", payload["unavailable"],
			)
		}
		return
	}
	copyPayload := centralStep3CopyMap(payload)
	raw, err := json.Marshal(copyPayload)
	if err != nil {
		if a.log != nil {
			a.log.Error("central read-model marshal failed", "snapshot_key", key, "error", err)
		}
		return
	}
	now := time.Now().UTC()
	if _, err := a.db.ExecContext(
		ctx,
		`INSERT INTO identity.central_screen_snapshots(snapshot_key,payload,updated_at)
		 VALUES($1,$2::jsonb,$3)
		 ON CONFLICT(snapshot_key) DO UPDATE
		 SET payload=EXCLUDED.payload,updated_at=EXCLUDED.updated_at`,
		key,
		string(raw),
		now,
	); err != nil {
		if a.log != nil {
			a.log.Error("central read-model persistence failed; retaining prior LKG", "snapshot_key", key, "error", err)
		}
		return
	}

	// Memory is a resilience mirror only. PostgreSQL is authoritative and must
	// commit first, otherwise a restart could regress to an older model.
	centralStep3Snapshots.Lock()
	centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
		payload: copyPayload, updatedAt: now,
	}
	centralStep3Snapshots.Unlock()
}

func (a *app) logCentralRefreshFailure(key string, unavailable []string) {
	if a.log == nil {
		return
	}
	a.log.Warn(
		"central background refresh failed; serving last-known-good snapshot",
		"snapshot_key", key,
		"unavailable", unavailable,
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

func centralStep3WaitBeginRefresh(ctx context.Context, key string) bool {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if centralStep3BeginRefresh(key) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
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
	var wg sync.WaitGroup
	for _, refresh := range refreshes {
		refresh := refresh
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.refreshCentralProjectionSerialized(refresh.key, refresh.fn)
		}()
	}
	wg.Wait()
}

func (a *app) refreshCentralStep3Registry() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	type result struct {
		kind  string
		page  central10ItemsPage
		err   error
	}
	results := make(chan result, 3)
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
	go func() {
		var page central10ItemsPage
		year := time.Now().UTC().Year()
		err := a.internalGET(ctx, a.hosts["catalog"], fmt.Sprintf("/internal/v1/module-usage-trend?year=%d", year), &page)
		results <- result{kind: "trend", page: page, err: err}
	}()

	var moduleDetails map[string]any
	var detailErr error
	var detailWG sync.WaitGroup
	detailWG.Add(1)
	go func() {
		defer detailWG.Done()
		detailErr = a.internalGET(ctx, a.hosts["catalog"], "/internal/v1/read-model/module-details", &moduleDetails)
	}()

	var modules, groups, trend []map[string]any
	var failed bool
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			failed = true
			continue
		}
		if res.kind == "modules" {
			modules = res.page.Items
		} else if res.kind == "groups" {
			groups = res.page.Items
		} else {
			trend = res.page.Items
		}
	}
	detailWG.Wait()
	if detailErr != nil {
		failed = true
	}
	if failed {
		a.logCentralRefreshFailure(centralStep3RegistryKey, []string{"catalog"})
		return
	}
	detailItems, _ := moduleDetails["items"].(map[string]any)
	if detailItems == nil {
		a.logCentralRefreshFailure(centralStep3RegistryKey, []string{"catalog_module_details"})
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3RegistryKey, map[string]any{
		"modules":        modules,
		"groups":         groups,
		"trend":          trend,
		"module_details": detailItems,
		"status":         "healthy",
		"unavailable":    []string{},
	})
}

func (a *app) refreshCentralStep3Plans() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	var plans, modules central10ItemsPage
	var plansErr, modulesErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		plansErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/plans", &plans)
	}()
	go func() {
		defer wg.Done()
		modulesErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules)
	}()
	wg.Wait()
	if plansErr != nil || modulesErr != nil {
		unavailable := []string{}
		if plansErr != nil { unavailable = append(unavailable, "billing_plans") }
		if modulesErr != nil { unavailable = append(unavailable, "catalog_modules") }
		a.logCentralRefreshFailure(centralStep3PlansKey, unavailable)
		return
	}

	persistCtx, persistCancel := context.WithTimeout(context.Background(), readModelPersistBudget)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3PlansKey, map[string]any{
		"plans": plans.Items,
		"modules": modules.Items,
		"status": "healthy",
		"unavailable": []string{},
	})
}

func (a *app) refreshCentralStep3Analytics() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	var analytics map[string]any
	var modules central10ItemsPage
	var analyticsErr, modulesErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		analyticsErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/packages/analytics", &analytics)
	}()
	go func() {
		defer wg.Done()
		modulesErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules)
	}()
	wg.Wait()
	if analyticsErr != nil || modulesErr != nil {
		unavailable := []string{}
		if analyticsErr != nil { unavailable = append(unavailable, "package_analytics") }
		if modulesErr != nil { unavailable = append(unavailable, "catalog_modules") }
		a.logCentralRefreshFailure(centralStep3AnalyticsKey, unavailable)
		return
	}

	persistCtx, persistCancel := context.WithTimeout(context.Background(), readModelPersistBudget)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep3AnalyticsKey, map[string]any{
		"analytics": analytics,
		"modules": modules.Items,
		"status": "healthy",
		"unavailable": []string{},
	})
}

func (a *app) refreshCentralStep3Commercial() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep3MaterializeBudget)
	defer cancel()

	partners, partnerErr := a.central10AllPartners(ctx)
	if partnerErr != nil {
		a.logCentralRefreshFailure(centralStep3CommercialKey, []string{"partners"})
		return
	}
	partnerIDs := make([]string, 0, len(partners))
	for _, partner := range partners {
		if id := central10String(partner["id"]); id != "" {
			partnerIDs = append(partnerIDs, id)
		}
	}
	sort.Strings(partnerIDs)

	var modules, plans central10ItemsPage
	var modulesErr, plansErr error
	var matrixItems, subscriptionItems []map[string]any
	var matrixErr, subscriptionsErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		matrixItems, subscriptionItems, matrixErr, subscriptionsErr = a.central10CommercialSources(ctx, partnerIDs, true)
	}()
	go func() {
		defer wg.Done()
		modulesErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules)
	}()
	go func() {
		defer wg.Done()
		plansErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/plans", &plans)
	}()
	wg.Wait()

	unavailable := []string{}
	if matrixErr != nil { unavailable = append(unavailable, "commercial_matrix") }
	if subscriptionsErr != nil { unavailable = append(unavailable, "subscription_matrix") }
	if modulesErr != nil { unavailable = append(unavailable, "catalog_modules") }
	if plansErr != nil { unavailable = append(unavailable, "billing_plans") }
	if len(unavailable) > 0 {
		a.logCentralRefreshFailure(centralStep3CommercialKey, unavailable)
		return
	}

	payload := map[string]any{
		"partners": partners,
		"matrix_items": matrixItems,
		"subscription_items": subscriptionItems,
		"modules": modules.Items,
		"plans": plans.Items,
		"matrix_available": true,
		"subscriptions_available": true,
		"status": "healthy",
		"unavailable": []string{},
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), readModelPersistBudget)
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
