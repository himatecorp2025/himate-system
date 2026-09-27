package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const (
	readModelTargetLatency      = 15 * time.Millisecond
	readModelRefreshPoll        = 2 * time.Second
	readModelRefreshBatch       = 100
	readModelPersistBudget      = 2 * time.Second
)

var readModelRefreshWake = make(chan struct{}, 1)

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
	return centralStep3SnapshotGet(key)
}

func partnerWorkspaceSnapshotValid(payload map[string]any) bool {
	if payload == nil || !strings.EqualFold(central10String(payload["status"]), "healthy") {
		return false
	}
	required := []string{
		"partner", "modules", "module_view", "production_environment",
		"preferred_connector_environment", "billing", "terms", "license",
		"documents", "invoices", "subscriptions", "environments",
		"provisioning_jobs", "impact_summary", "evidence",
		"connector_credentials", "portal_users", "agreement",
		"commercial_status", "billing_events", "website_adapter",
		"partner_design", "payment_profile", "portal_gate",
		"portal_modules", "portal_plans", "portal_plan", "portal_plan_modules",
		"portal_charity", "portal_charity_modules", "portal_design_media",
		"portal_billing_subscriptions", "portal_billing_invoices",
		"portal_user_module_policies", "portal_notifications", "tenant_finance",
		"partner_audit_events",
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

func (a *app) enqueueReadModelRefresh(ctx context.Context, reason, partnerID string) {
	partnerID = strings.TrimSpace(partnerID)
	if _, err := a.db.ExecContext(ctx,
		`INSERT INTO identity.read_model_refresh_queue(scope,partner_id,reason)
		 VALUES('all',$1,$2)`,
		partnerID, strings.TrimSpace(reason),
	); err != nil {
		if a.log != nil {
			a.log.Error("read-model refresh event persistence failed", "partner_id", partnerID, "reason", reason, "error", err)
		}
		return
	}
	select {
	case readModelRefreshWake <- struct{}{}:
	default:
	}
}

func readModelReasonRefreshesAllTenants(reason string) bool {
	reason = strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(reason, "/modules") ||
		strings.Contains(reason, "/module-groups") ||
		strings.Contains(reason, "/billing/plans") ||
		strings.Contains(reason, "/cms/design")
}

func (a *app) refreshReadModelsForEvent(partnerID, reason string, createdAt time.Time) bool {
	partnerID = strings.TrimSpace(partnerID)
	var wg sync.WaitGroup
	for _, job := range a.centralReadinessJobs() {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			job.refresh()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.refreshDashboardSnapshot(time.Now().UTC().Year())
	}()
	if partnerID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceMaterializeBudget)
			defer cancel()
			a.refreshCentralPartnerWorkspace(ctx, partnerID)
		}()
	}
	if readModelReasonRefreshesAllTenants(reason) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceStartupBudget)
			defer cancel()
			a.refreshCentralPartnerWorkspaceSnapshots(ctx, false)
		}()
	}
	wg.Wait()

	verifyCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, job := range a.centralReadinessJobs() {
		_, updated, err := a.loadCentralSnapshotDB(verifyCtx, job.key)
		if err != nil || updated.Before(createdAt) {
			return false
		}
	}
	_, dashboardUpdated, err := a.loadDashboardSnapshotContext(verifyCtx, time.Now().UTC().Year())
	if err != nil || dashboardUpdated.Before(createdAt) {
		return false
	}
	if partnerID != "" {
		_, updated, err := a.loadPartnerWorkspaceDB(verifyCtx, partnerID)
		if err != nil || updated.Before(createdAt) {
			return false
		}
	}
	return true
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
	type event struct {
		id        int64
		partnerID string
		reason    string
		createdAt time.Time
	}
	events := []event{}
	for rows.Next() {
		var item event
		if rows.Scan(&item.id, &item.partnerID, &item.reason, &item.createdAt) == nil {
			events = append(events, item)
		}
	}
	rows.Close()

	for _, item := range events {
		if !a.refreshReadModelsForEvent(item.partnerID, item.reason, item.createdAt.UTC()) {
			if a.log != nil {
				a.log.Warn("read-model refresh event remains pending", "event_id", item.id, "reason", item.reason, "partner_id", item.partnerID)
			}
			continue
		}
		if _, err := a.db.Exec(
			`UPDATE identity.read_model_refresh_queue SET processed_at=NOW() WHERE id=$1`,
			item.id,
		); err != nil && a.log != nil {
			a.log.Error("read-model refresh event acknowledgement failed", "event_id", item.id, "error", err)
		}
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
