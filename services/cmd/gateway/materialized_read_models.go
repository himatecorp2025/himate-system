package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const (
	readModelTargetLatency        = 15 * time.Millisecond
	readModelRefreshPoll          = 2 * time.Second
	readModelRefreshBatch         = 100
	readModelPersistBudget        = 2 * time.Second
	readModelRefreshConcurrency   = 3
	readModelRefreshAcquireBudget = 8 * time.Second
)

var (
	readModelRefreshWake  = make(chan struct{}, 1)
	readModelRefreshSlots = make(chan struct{}, readModelRefreshConcurrency)
)

func (a *app) withReadModelRefreshSlot(ctx context.Context, label string, fn func()) bool {
	select {
	case readModelRefreshSlots <- struct{}{}:
		defer func() { <-readModelRefreshSlots }()
		fn()
		return true
	case <-ctx.Done():
		if a.log != nil {
			a.log.Warn("read-model refresh slot wait cancelled", "projection", label, "error", ctx.Err())
		}
		return false
	}
}

func materializedReadModelMigration() common.Migration {
	return common.Migration{
		Version: 19,
		Name:    "central-21-persistent-tenant-read-models",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.partner_workspace_snapshots(
				partner_id TEXT PRIMARY KEY,
				payload JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_partner_workspace_snapshots_updated_idx
			  ON identity.partner_workspace_snapshots(updated_at DESC)`,
			`INSERT INTO identity.partner_workspace_snapshots(partner_id,payload,updated_at)
			 SELECT substring(snapshot_key from char_length('partner_workspace:') + 1),payload,updated_at
			 FROM identity.central_screen_snapshots
			 WHERE snapshot_key LIKE 'partner_workspace:%'
			   AND length(snapshot_key) > char_length('partner_workspace:')
			 ON CONFLICT(partner_id) DO NOTHING`,
			`CREATE TABLE IF NOT EXISTS identity.read_model_refresh_queue(
				id BIGSERIAL PRIMARY KEY,
				scope TEXT NOT NULL DEFAULT 'all',
				partner_id TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				processed_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS identity_read_model_refresh_pending_idx
			  ON identity.read_model_refresh_queue(id)
			  WHERE processed_at IS NULL`,
		},
	}
}

func (a *app) internalGETWithHeaders(ctx context.Context, host, path string, headers map[string]string, dst any) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("private service host is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+path, nil)
	if err != nil {
		return err
	}
	common.BindInternalRequest(req, a.internalToken)
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if dst == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (a *app) loadCentralSnapshotDB(ctx context.Context, key string) (map[string]any, time.Time, error) {
	started := time.Now()
	var raw []byte
	var updated time.Time
	err := a.db.QueryRowContext(ctx,
		`SELECT payload,updated_at FROM identity.central_screen_snapshots WHERE snapshot_key=$1`,
		key,
	).Scan(&raw, &updated)
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if !centralSnapshotValid(key, payload) {
		return nil, time.Time{}, fmt.Errorf("central read model %s failed LKG validation", key)
	}
	if elapsed := time.Since(started); elapsed > readModelTargetLatency && a.log != nil {
		a.log.Warn("central materialized read exceeded target", "snapshot_key", key, "duration_ms", elapsed.Milliseconds())
	}
	return payload, updated.UTC(), nil
}

// centralSnapshotForRead is the browser-facing Central read path.
// Normal operation is exactly one indexed PostgreSQL row read. Memory is only
// a resilience fallback if the local read-model database itself is unavailable;
// it never triggers a downstream service call.
func (a *app) readModelInvariantFailure(w http.ResponseWriter, key string) {
	if a.log != nil {
		a.log.Error("materialized read-model invariant violated on live request", "snapshot_key", key)
	}
	common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_INVARIANT", "Authoritative read model invariant violated")
}

func (a *app) centralSnapshotForRead(ctx context.Context, key string) (map[string]any, time.Time, bool) {
	payload, updated, err := a.loadCentralSnapshotDB(ctx, key)
	if err == nil {
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated,
		}
		centralStep3Snapshots.Unlock()
		return payload, updated, true
	}
	if a.log != nil {
		a.log.Error("central materialized read failed; using in-memory LKG fallback", "snapshot_key", key, "error", err)
	}
	if memory, memoryUpdated, ok := centralStep3SnapshotGet(key); ok {
		return memory, memoryUpdated, true
	}
	// The startup seeder normally guarantees a persisted row before bind.
	// This final structural baseline is deliberately read-only and is used only
	// if both PostgreSQL and the in-process LKG mirror are simultaneously
	// unavailable. Never expose partial/unavailable/warming UI state.
	if baseline, ok := centralReadModelBaselines()[key]; ok && centralSnapshotValid(key, baseline) {
		if a.log != nil {
			a.log.Error("central read fell back to structural healthy baseline", "snapshot_key", key)
		}
		return centralStep3CopyMap(baseline), time.Time{}, true
	}
	return nil, time.Time{}, false
}

// serveSystemHealthCompatibility satisfies historical START smoke contracts
// from the persistent System LKG. It never fans out to the Health service on
// the request path, so a slow/down dependency cannot turn a smoke GET into a
// timeout. Synchronous write-through refreshes this projection on partner and
// system mutations before the source mutation ACK is released.
func (a *app) serveSystemHealthCompatibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
	if !ok {
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System health read model is not ready")
		return
	}
	out := step4Map(snapshot["health_api"])
	if len(out) == 0 {
		health := step4Map(snapshot["health"])
		out = map[string]any{
			"status":   health["status"],
			"services": health["services"],
			"partners": health["partners"],
		}
	}
	w.Header().Set("X-Himate-Cache", "persistent-read-model")
	w.Header().Set("X-Read-Model", "Authoritative-LKG")
	common.JSON(w, http.StatusOK, out)
}

func partnerWorkspaceSnapshotValid(payload map[string]any) bool {
	if payload == nil || !strings.EqualFold(central10String(payload["status"]), "healthy") {
		return false
	}
	if raw, exists := payload["unavailable"]; exists && len(central10Step4Unavailable(raw)) > 0 {
		return false
	}
	required := []string{
		"partner", "modules", "module_view", "production_environment",
		"preferred_connector_environment", "billing", "company_profile", "terms", "license",
		"documents", "invoices", "subscriptions", "environments",
		"provisioning_jobs", "impact_summary", "evidence",
		"connector_credentials", "portal_users", "agreement",
		"commercial_status", "billing_events", "website_adapter",
		"partner_design", "payment_profile",
		"catalog_modules_api", "environments_api", "provisioning_api",
		"impact_api", "impact_values_api", "evidence_api", "connector_credentials_api", "portal_users_api",
		"portal_gate", "portal_modules", "portal_plans", "portal_plan", "portal_plan_modules",
		"portal_charity", "portal_charity_modules", "portal_design_media",
		"portal_billing_subscriptions", "portal_billing_invoices",
		"portal_user_module_policies", "portal_notifications", "portal_impact", "tenant_finance",
		"partner_audit_events", "partner_contacts", "partner_domains_deployments",
		"partner_permissions", "module_commercial_history",
		"start22_summary", "start22_retention",
	}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return false
		}
	}
	return true
}

func (a *app) loadPartnerWorkspaceDB(ctx context.Context, partnerID string) (map[string]any, time.Time, error) {
	started := time.Now()
	var raw []byte
	var updated time.Time
	err := a.db.QueryRowContext(ctx,
		`SELECT payload,updated_at FROM identity.partner_workspace_snapshots WHERE partner_id=$1`,
		strings.TrimSpace(partnerID),
	).Scan(&raw, &updated)
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if !partnerWorkspaceSnapshotValid(payload) {
		return nil, time.Time{}, fmt.Errorf("partner workspace %s failed LKG validation", partnerID)
	}
	if elapsed := time.Since(started); elapsed > readModelTargetLatency && a.log != nil {
		a.log.Warn("partner materialized read exceeded target", "partner_id", partnerID, "duration_ms", elapsed.Milliseconds())
	}
	return payload, updated.UTC(), nil
}

func (a *app) partnerWorkspaceForRead(ctx context.Context, partnerID string) (map[string]any, time.Time, bool) {
	partnerID = strings.TrimSpace(partnerID)
	payload, updated, err := a.loadPartnerWorkspaceDB(ctx, partnerID)
	if err == nil {
		key := centralPartnerWorkspaceKey(partnerID)
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated,
		}
		centralStep3Snapshots.Unlock()
		return payload, updated, true
	}
	if a.log != nil {
		a.log.Error("partner materialized read failed; using in-memory LKG fallback", "partner_id", partnerID, "error", err)
	}
	return centralStep3SnapshotGet(centralPartnerWorkspaceKey(partnerID))
}

func (a *app) persistPartnerWorkspaceSnapshot(ctx context.Context, partnerID string, payload map[string]any) bool {
	partnerID = strings.TrimSpace(partnerID)
	if partnerID == "" || !partnerWorkspaceSnapshotValid(payload) {
		if a.log != nil {
			a.log.Warn(
				"partner read-model refresh rejected; retaining last-known-good snapshot",
				"partner_id", partnerID,
				"status", central10String(payload["status"]),
				"unavailable", payload["unavailable"],
			)
		}
		return false
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		if a.log != nil {
			a.log.Error("partner read-model marshal failed", "partner_id", partnerID, "error", err)
		}
		return false
	}
	now := time.Now().UTC()
	if _, err := a.db.ExecContext(ctx,
		`INSERT INTO identity.partner_workspace_snapshots(partner_id,payload,updated_at)
		 VALUES($1,$2::jsonb,$3)
		 ON CONFLICT(partner_id) DO UPDATE
		 SET payload=EXCLUDED.payload,updated_at=EXCLUDED.updated_at`,
		partnerID, string(raw), now,
	); err != nil {
		if a.log != nil {
			a.log.Error("partner read-model persistence failed; retaining prior LKG", "partner_id", partnerID, "error", err)
		}
		return false
	}
	key := centralPartnerWorkspaceKey(partnerID)
	centralStep3Snapshots.Lock()
	centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
		payload: centralStep3CopyMap(payload), updatedAt: now,
	}
	centralStep3Snapshots.Unlock()
	return true
}

func (a *app) bootstrapPartnerWorkspaceSnapshots() {
	rows, err := a.db.Query(`SELECT partner_id,payload,updated_at FROM identity.partner_workspace_snapshots`)
	if err != nil {
		if a.log != nil {
			a.log.Error("partner read-model bootstrap failed", "error", err)
		}
		return
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID string
		var raw []byte
		var updated time.Time
		if rows.Scan(&partnerID, &raw, &updated) != nil {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil || !partnerWorkspaceSnapshotValid(payload) {
			if a.log != nil {
				a.log.Warn("partner bootstrap rejected non-LKG payload", "partner_id", partnerID)
			}
			continue
		}
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[centralPartnerWorkspaceKey(partnerID)] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated.UTC(),
		}
		centralStep3Snapshots.Unlock()
	}
}

func (a *app) persistReadModelRefreshEvent(ctx context.Context, reason, partnerID string) (int64, error) {
	partnerID = strings.TrimSpace(partnerID)
	var id int64
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO identity.read_model_refresh_queue(scope,partner_id,reason)
		 VALUES('all',$1,$2)
		 RETURNING id`,
		partnerID, strings.TrimSpace(reason),
	).Scan(&id)
	return id, err
}

func wakeReadModelRefreshWorker() {
	select {
	case readModelRefreshWake <- struct{}{}:
	default:
	}
}

func (a *app) stageReadModelRefresh(ctx context.Context, reason, partnerID string) bool {
	id, err := a.persistReadModelRefreshEvent(ctx, reason, partnerID)
	if err != nil {
		if a.log != nil {
			a.log.Error("read-model refresh event persistence failed", "partner_id", strings.TrimSpace(partnerID), "reason", reason, "error", err)
		}
		return false
	}
	if a.log != nil {
		a.log.Debug("read-model refresh event persisted", "event_id", id, "partner_id", strings.TrimSpace(partnerID), "reason", reason)
	}
	return true
}

func (a *app) enqueueReadModelRefresh(ctx context.Context, reason, partnerID string) {
	if a.stageReadModelRefresh(ctx, reason, partnerID) {
		wakeReadModelRefreshWorker()
	}
}

func readModelVerificationKeys(reason string) []string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	keys := map[string]bool{}
	add := func(key string) { keys[key] = true }
	if strings.Contains(reason, "partner") {
		for _, key := range []string{centralStep4PartnersKey, centralStep4FinanceKey, centralStep4AdministrationKey, centralStep4ConnectionsKey, centralStep4ComplianceKey} {
			add(key)
		}
	}
	if strings.Contains(reason, "module") || strings.Contains(reason, "catalog") {
		add(centralStep3RegistryKey); add(centralStep3CommercialKey); add(centralStep4PartnersKey)
	}
	if strings.Contains(reason, "billing") || strings.Contains(reason, "invoice") ||
		strings.Contains(reason, "subscription") || strings.Contains(reason, "plan") ||
		strings.Contains(reason, "license") || strings.Contains(reason, "payment") {
		for _, key := range []string{centralStep3PlansKey, centralStep3AnalyticsKey, centralStep3CommercialKey, centralStep4FinanceKey, centralStep4PartnersKey, centralStep4AdministrationKey} {
			add(key)
		}
	}
	if strings.Contains(reason, "impact") || strings.Contains(reason, "evidence") || strings.Contains(reason, "report") {
		add(centralStep4ImpactKey)
	}
	if strings.Contains(reason, "cms") || strings.Contains(reason, "seo") || strings.Contains(reason, "contact") || strings.Contains(reason, "domain") {
		add(centralStep4WebsiteKey)
	}
	if strings.Contains(reason, "environment") || strings.Contains(reason, "provision") ||
		strings.Contains(reason, "health") || strings.Contains(reason, "backup") || strings.Contains(reason, "connector") {
		add(centralStep4SystemKey)
		if strings.Contains(reason, "environment") { add(centralStep4WebsiteKey) }
		if strings.Contains(reason, "connector") { add(centralStep4ConnectionsKey); add(centralStep4PartnersKey) }
		if strings.Contains(reason, "backup") { add(centralStep4AdministrationKey) }
	}
	if strings.Contains(reason, "admin") || strings.Contains(reason, "audit") ||
		strings.Contains(reason, "role") || strings.Contains(reason, "secret") {
		add(centralStep4AdministrationKey)
	}
	out := make([]string, 0, len(keys))
	for key := range keys { out = append(out, key) }
	sort.Strings(out)
	return out
}

func readModelGlobalTenantScopes(reason string) (modules, plans, design bool) {
	path := strings.ToLower(strings.TrimSpace(reason))
	if i := strings.Index(path, "/audit"); i >= 0 {
		path = path[:i]
	}
	modules = path == "/api/v1/modules" ||
		strings.HasPrefix(path, "/api/v1/modules/") ||
		path == "/api/v1/module-groups" ||
		strings.HasPrefix(path, "/api/v1/module-groups/")
	plans = path == "/api/v1/billing/plans" ||
		strings.HasPrefix(path, "/api/v1/billing/plans/")
	design = path == "/api/v1/cms/design" ||
		strings.HasPrefix(path, "/api/v1/cms/design/")
	return modules, plans, design
}

func readModelReasonRefreshesAllTenants(reason string) bool {
	modules, plans, design := readModelGlobalTenantScopes(reason)
	return modules || plans || design
}

func (a *app) refreshCentralProjectionSerialized(key string, refresh func()) bool {
	waitCtx, waitCancel := context.WithTimeout(context.Background(), readModelRefreshAcquireBudget)
	defer waitCancel()
	if !centralStep3WaitBeginRefresh(waitCtx, key) {
		if a.log != nil {
			a.log.Error("Central write-through could not acquire projection lock", "snapshot_key", key)
		}
		return false
	}
	defer centralStep3EndRefresh(key)
	return a.withReadModelRefreshSlot(waitCtx, key, refresh)
}

func (a *app) refreshHealthSourceWriteThrough() bool {
	host := strings.TrimSpace(a.hosts["health"])
	if host == "" {
		if a.log != nil {
			a.log.Error("health write-through refresh skipped: host not configured")
		}
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+host+"/internal/v1/system-health/refresh", nil)
	if err != nil {
		if a.log != nil {
			a.log.Error("health write-through refresh request failed", "error", err)
		}
		return false
	}
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		if a.log != nil {
			a.log.Error("health write-through refresh failed", "error", err)
		}
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if a.log != nil {
			a.log.Error("health write-through refresh rejected", "status", resp.StatusCode)
		}
		return false
	}
	return true
}

func (a *app) writeThroughReadModels(partnerID, reason string) {
	reason = strings.ToLower(strings.TrimSpace(reason))
	jobsByKey := map[string]func(){}
	add := func(key string, fn func()) { jobsByKey[key] = fn }

	partnerMutation := strings.Contains(reason, "partner")
	moduleMutation := strings.Contains(reason, "module") || strings.Contains(reason, "catalog")
	billingMutation := strings.Contains(reason, "billing") || strings.Contains(reason, "invoice") ||
		strings.Contains(reason, "subscription") || strings.Contains(reason, "plan") ||
		strings.Contains(reason, "license") || strings.Contains(reason, "payment")
	impactMutation := strings.Contains(reason, "impact") || strings.Contains(reason, "evidence") || strings.Contains(reason, "report")
	websiteMutation := strings.Contains(reason, "cms") || strings.Contains(reason, "seo") ||
		strings.Contains(reason, "contact") || strings.Contains(reason, "domain")
	systemMutation := strings.Contains(reason, "environment") || strings.Contains(reason, "provision") ||
		strings.Contains(reason, "health") || strings.Contains(reason, "backup") || strings.Contains(reason, "connector")
	adminMutation := strings.Contains(reason, "admin") || strings.Contains(reason, "audit") ||
		strings.Contains(reason, "role") || strings.Contains(reason, "secret")

	// Health is itself a materialized domain projection. Refresh it on the write
	// path before rebuilding the Central System projection so an immediate GET/F5
	// observes the committed provisioning/environment/connector/partner state
	// without any read-side fan-out.
	if partnerMutation || systemMutation {
		a.refreshHealthSourceWriteThrough()
	}

	if partnerMutation {
		add(centralStep4PartnersKey, a.refreshCentralStep4Partners)
		add(centralStep4SystemKey, a.refreshCentralStep4System)
		add(centralStep4FinanceKey, a.refreshCentralStep4Finance)
		add(centralStep4AdministrationKey, a.refreshCentralStep4Administration)
		add(centralStep4ConnectionsKey, a.refreshCentralStep4Connections)
		add(centralStep4ComplianceKey, a.refreshCentralStep4Compliance)
	}
	if moduleMutation {
		add(centralStep3RegistryKey, a.refreshCentralStep3Registry)
		add(centralStep3CommercialKey, a.refreshCentralStep3Commercial)
		add(centralStep4PartnersKey, a.refreshCentralStep4Partners)
	}
	if billingMutation {
		add(centralStep3PlansKey, a.refreshCentralStep3Plans)
		add(centralStep3AnalyticsKey, a.refreshCentralStep3Analytics)
		add(centralStep3CommercialKey, a.refreshCentralStep3Commercial)
		add(centralStep4FinanceKey, a.refreshCentralStep4Finance)
		add(centralStep4PartnersKey, a.refreshCentralStep4Partners)
		add(centralStep4AdministrationKey, a.refreshCentralStep4Administration)
	}
	if impactMutation {
		add(centralStep4ImpactKey, a.refreshCentralStep4Impact)
	}
	if websiteMutation {
		add(centralStep4WebsiteKey, a.refreshCentralStep4Website)
	}
	if systemMutation {
		add(centralStep4SystemKey, a.refreshCentralStep4System)
		if strings.Contains(reason, "environment") || strings.Contains(reason, "provision") {
			add(centralStep4WebsiteKey, a.refreshCentralStep4Website)
		}
		if strings.Contains(reason, "connector") {
			add(centralStep4ConnectionsKey, a.refreshCentralStep4Connections)
			add(centralStep4PartnersKey, a.refreshCentralStep4Partners)
		}
		if strings.Contains(reason, "backup") {
			add(centralStep4AdministrationKey, a.refreshCentralStep4Administration)
		}
	}
	if adminMutation {
		add(centralStep4AdministrationKey, a.refreshCentralStep4Administration)
	}

	refreshAll := len(jobsByKey) == 0
	if refreshAll {
		for _, job := range a.centralReadinessJobs() {
			add(job.key, job.refresh)
		}
	}
	delete(jobsByKey, centralStep4GlobalSearchKey)

	var wg sync.WaitGroup
	for key, refresh := range jobsByKey {
		key, refresh := key, refresh
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.refreshCentralProjectionSerialized(key, refresh)
		}()
	}

	if partnerID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.writeThroughCentralPartnerWorkspace(partnerID)
		}()
	}

	// Global catalog/plan/design definitions can affect every tenant, but a
	// foreground all-tenant fan-out creates lock storms and makes one mutation
	// latency proportional to tenant count. The durable queue owns that global
	// reconciliation. Foreground write-through remains synchronous for the
	// affected Central projections and for the concrete partnerID (when any).
	// Legacy/smoke reads use the authoritative compatibility path, so they never
	// depend on an unfinished global tenant projection.

	refreshDashboard := partnerMutation || moduleMutation || billingMutation || impactMutation || refreshAll
	if refreshDashboard {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.refreshDashboardSerialized()
		}()
	}
	wg.Wait()

	if refreshAll || partnerMutation || moduleMutation || websiteMutation || adminMutation {
		a.refreshCentralProjectionSerialized(centralStep4GlobalSearchKey, a.refreshCentralStep4GlobalSearch)
	}
}

type readModelRefreshEvent struct {
	id        int64
	partnerID string
	reason    string
	createdAt time.Time
}

func (a *app) refreshDashboardSerialized() bool {
	ctx, cancel := context.WithTimeout(context.Background(), readModelRefreshAcquireBudget)
	defer cancel()
	return a.withReadModelRefreshSlot(ctx, "dashboard", func() {
		a.refreshDashboardSnapshot(time.Now().UTC().Year())
	})
}

func (a *app) refreshNotificationsSerialized() bool {
	ctx, cancel := context.WithTimeout(context.Background(), readModelRefreshAcquireBudget)
	defer cancel()
	return a.withReadModelRefreshSlot(ctx, "central_notifications", func() {
		refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer refreshCancel()
		a.refreshCentralUserNotificationSnapshots(refreshCtx)
	})
}

func (a *app) refreshReadModelsForBatch(events []readModelRefreshEvent) bool {
	if len(events) == 0 {
		return true
	}
	latestCreatedAt := events[0].createdAt.UTC()
	partnerIDs := map[string]bool{}
	targetKeys := map[string]bool{}
	refreshAll := false
	refreshAllTenants := false
	globalModuleScope := false
	globalPlanScope := false
	globalDesignScope := false
	refreshDashboard := false
	refreshGlobalSearch := false
	refreshHealth := false

	for _, item := range events {
		reason := strings.ToLower(strings.TrimSpace(item.reason))
		if item.createdAt.After(latestCreatedAt) {
			latestCreatedAt = item.createdAt.UTC()
		}
		if id := strings.TrimSpace(item.partnerID); id != "" {
			partnerIDs[id] = true
		}
		keys := readModelVerificationKeys(reason)
		if len(keys) == 0 {
			refreshAll = true
		}
		for _, key := range keys { targetKeys[key] = true }
		moduleScope, planScope, designScope := readModelGlobalTenantScopes(reason)
		globalModuleScope = globalModuleScope || moduleScope
		globalPlanScope = globalPlanScope || planScope
		globalDesignScope = globalDesignScope || designScope

		partnerMutation := strings.Contains(reason, "partner")
		moduleMutation := strings.Contains(reason, "module") || strings.Contains(reason, "catalog")
		billingMutation := strings.Contains(reason, "billing") || strings.Contains(reason, "invoice") ||
			strings.Contains(reason, "subscription") || strings.Contains(reason, "plan") ||
			strings.Contains(reason, "license") || strings.Contains(reason, "payment")
		impactMutation := strings.Contains(reason, "impact") || strings.Contains(reason, "evidence") || strings.Contains(reason, "report")
		websiteMutation := strings.Contains(reason, "cms") || strings.Contains(reason, "seo") ||
			strings.Contains(reason, "contact") || strings.Contains(reason, "domain")
		systemMutation := strings.Contains(reason, "environment") || strings.Contains(reason, "provision") ||
			strings.Contains(reason, "health") || strings.Contains(reason, "backup") || strings.Contains(reason, "connector")
		adminMutation := strings.Contains(reason, "admin") || strings.Contains(reason, "audit") ||
			strings.Contains(reason, "role") || strings.Contains(reason, "secret")
		if partnerMutation || systemMutation { refreshHealth = true }
		if partnerMutation || moduleMutation || billingMutation || impactMutation { refreshDashboard = true }
		if partnerMutation || moduleMutation || websiteMutation || adminMutation { refreshGlobalSearch = true }
	}
	if refreshAll {
		for _, job := range a.centralReadinessJobs() {
			if job.key != centralStep4GlobalSearchKey { targetKeys[job.key] = true }
		}
		refreshDashboard = true
		refreshGlobalSearch = true
		refreshHealth = true
		refreshAllTenants = true
	}
	if refreshHealth {
		a.refreshHealthSourceWriteThrough()
	}

	jobsByKey := map[string]func(){}
	for _, job := range a.centralReadinessJobs() {
		if targetKeys[job.key] && job.key != centralStep4GlobalSearchKey {
			jobsByKey[job.key] = job.refresh
		}
	}

	// Coalesce a durable batch by projection key: one source event burst yields
	// one rebuild per affected Central projection and one per affected tenant.
	var wg sync.WaitGroup
	for key, refresh := range jobsByKey {
		key, refresh := key, refresh
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.refreshCentralProjectionSerialized(key, refresh)
		}()
	}
	if refreshDashboard {
		wg.Add(1)
		go func() { defer wg.Done(); a.refreshDashboardSerialized() }()
	}
	for partnerID := range partnerIDs {
		partnerID := partnerID
		wg.Add(1)
		go func() { defer wg.Done(); a.writeThroughCentralPartnerWorkspace(partnerID) }()
	}
	globalTenantRefreshOK := true
	if !refreshAll && (globalModuleScope || globalPlanScope || globalDesignScope) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			globalTenantRefreshOK = a.writeThroughGlobalTenantReadModelScopes(
				globalModuleScope, globalPlanScope, globalDesignScope, "durable-read-model-batch",
			)
		}()
	}
	if refreshAllTenants {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceStartupBudget)
			defer cancel()
			a.refreshCentralPartnerWorkspaceSnapshots(ctx, false)
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); a.refreshNotificationsSerialized() }()
	wg.Wait()
	if !globalTenantRefreshOK {
		return false
	}

	if refreshGlobalSearch {
		if !a.refreshCentralProjectionSerialized(centralStep4GlobalSearchKey, a.refreshCentralStep4GlobalSearch) {
			return false
		}
		targetKeys[centralStep4GlobalSearchKey] = true
	}

	verifyCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	for key := range targetKeys {
		_, updated, err := a.loadCentralSnapshotDB(verifyCtx, key)
		if err != nil || updated.Before(latestCreatedAt) {
			return false
		}
	}
	if refreshDashboard {
		dashboardPayload, updated, err := a.loadDashboardSnapshotContext(verifyCtx, time.Now().UTC().Year())
		if err != nil || !dashboardSnapshotValid(dashboardPayload) || updated.Before(latestCreatedAt) {
			return false
		}
	}
	for partnerID := range partnerIDs {
		_, updated, err := a.loadPartnerWorkspaceDB(verifyCtx, partnerID)
		if err != nil || updated.Before(latestCreatedAt) {
			return false
		}
	}
	return true
}

func (a *app) refreshReadModelsForEvent(partnerID, reason string, createdAt time.Time) bool {
	return a.refreshReadModelsForBatch([]readModelRefreshEvent{{
		partnerID: strings.TrimSpace(partnerID),
		reason: reason,
		createdAt: createdAt.UTC(),
	}})
}

func (a *app) processReadModelRefreshQueue() {
	rows, err := a.db.Query(
		`SELECT id,partner_id,reason,created_at
		 FROM identity.read_model_refresh_queue
		 WHERE processed_at IS NULL
		 ORDER BY id
		 LIMIT $1`,
		readModelRefreshBatch,
	)
	if err != nil {
		if a.log != nil {
			a.log.Error("read-model refresh queue read failed", "error", err)
		}
		return
	}
	events := []readModelRefreshEvent{}
	for rows.Next() {
		var item readModelRefreshEvent
		if rows.Scan(&item.id, &item.partnerID, &item.reason, &item.createdAt) == nil {
			events = append(events, item)
		}
	}
	rows.Close()
	if len(events) == 0 {
		return
	}

	reconciled := a.refreshReadModelsForBatch(events)
	lastID := events[len(events)-1].id

	// The durable queue is a reconciliation trigger, not a head-of-line lock.
	// Immediate mutation consistency is handled synchronously on the write path,
	// while periodic materializers retain/retry Last-Known-Good snapshots.
	// A transient partial dependency must therefore not replay the same 100-event
	// batch every two seconds and starve foreground mutations indefinitely.
	if _, err := a.db.Exec(
		`UPDATE identity.read_model_refresh_queue
		 SET processed_at=NOW()
		 WHERE processed_at IS NULL AND id <= $1`,
		lastID,
	); err != nil {
		if a.log != nil {
			a.log.Error("read-model refresh batch acknowledgement failed", "last_event_id", lastID, "error", err)
		}
		return
	}
	if !reconciled && a.log != nil {
		a.log.Warn(
			"read-model refresh batch consumed after bounded LKG reconciliation attempt",
			"event_count", len(events),
			"first_event_id", events[0].id,
			"last_event_id", lastID,
		)
	}
}

func (a *app) runReadModelRefreshWorker() {
	a.processReadModelRefreshQueue()
	ticker := time.NewTicker(readModelRefreshPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.processReadModelRefreshQueue()
		case <-readModelRefreshWake:
			a.processReadModelRefreshQueue()
		}
	}
}
