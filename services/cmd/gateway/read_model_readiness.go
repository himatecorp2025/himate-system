package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type centralReadinessJob struct {
	key     string
	refresh func()
}

var gatewayReadiness atomic.Bool

// gatewayReadinessGate is a final defensive guard in addition to the startup
// bind ordering. Normal traffic cannot enter the Gateway until the persistent
// LKG baseline and its in-process mirror have been established.
func gatewayReadinessGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := ""
		if r != nil && r.URL != nil {
			path = r.URL.Path
		}
		if gatewayReadiness.Load() || path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", "1")
		http.Error(w, "System warming up, readiness gate active", http.StatusServiceUnavailable)
	})
}

func (a *app) centralReadinessJobs() []centralReadinessJob {
	return []centralReadinessJob{
		{centralStep3RegistryKey, a.refreshCentralStep3Registry},
		{centralStep3PlansKey, a.refreshCentralStep3Plans},
		{centralStep3AnalyticsKey, a.refreshCentralStep3Analytics},
		{centralStep3CommercialKey, a.refreshCentralStep3Commercial},
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
}

func (a *app) ensureDashboardReadModelReady(ctx context.Context) error {
	year := time.Now().UTC().Year()
	payload, _, err := a.loadDashboardSnapshotContext(ctx, year)
	if err == nil && dashboardSnapshotValid(payload) && !readModelSeeded(payload) {
		return nil
	}
	// Seed rows make the first DB read deterministic, but they never suppress an
	// eager attempt to build the real projection before the public port opens.
	a.refreshDashboardSnapshot(year)
	payload, _, err = a.loadDashboardSnapshotContext(ctx, year)
	if err != nil {
		return err
	}
	if !dashboardSnapshotValid(payload) {
		return fmt.Errorf("dashboard read model is not Last-Known-Good")
	}
	if readModelSeeded(payload) && a.log != nil {
		a.log.Warn("dashboard is using healthy cold-start baseline until background projection succeeds", "year", year)
	}
	return nil
}

func (a *app) ensureCentralReadModelsReady(ctx context.Context) error {
	missing := []string{}
	for _, job := range a.centralReadinessJobs() {
		payload, _, err := a.loadCentralSnapshotDB(ctx, job.key)
		if err == nil && !readModelSeeded(payload) {
			continue
		}
		job.refresh()
		payload, _, err = a.loadCentralSnapshotDB(ctx, job.key)
		if err != nil {
			missing = append(missing, job.key)
			continue
		}
		if readModelSeeded(payload) && a.log != nil {
			a.log.Warn("Central screen is using healthy cold-start baseline until background projection succeeds", "snapshot_key", job.key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Central read models not ready: %s", strings.Join(missing, ","))
	}
	return nil
}

func (a *app) ensurePartnerReadModelsReady(ctx context.Context) error {
	partners, _, err := a.loadCentralSnapshotDB(ctx, centralStep4PartnersKey)
	if err != nil {
		return fmt.Errorf("partner registry read model unavailable: %w", err)
	}
	missing := []string{}
	for _, item := range step4Items(partners["items"]) {
		partnerID := central10String(item["id"])
		if partnerID == "" {
			continue
		}
		payload, _, loadErr := a.loadPartnerWorkspaceDB(ctx, partnerID)
		if loadErr != nil {
			if seedErr := a.seedPartnerWorkspaceBaseline(ctx, item); seedErr != nil {
				missing = append(missing, partnerID)
				continue
			}
			payload, _, loadErr = a.loadPartnerWorkspaceDB(ctx, partnerID)
		}
		if loadErr == nil && !readModelSeeded(payload) {
			continue
		}
		partnerCtx, cancel := context.WithTimeout(ctx, centralPartnerWorkspaceMaterializeBudget)
		a.refreshCentralPartnerWorkspace(partnerCtx, partnerID)
		cancel()
		payload, _, loadErr = a.loadPartnerWorkspaceDB(ctx, partnerID)
		if loadErr != nil {
			missing = append(missing, partnerID)
			continue
		}
		if readModelSeeded(payload) && a.log != nil {
			a.log.Warn("tenant workspace is using healthy cold-start baseline until background projection succeeds", "partner_id", partnerID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("tenant read models not ready: %s", strings.Join(missing, ","))
	}
	return nil
}

// ensureMaterializedReadModelsReady runs before common.Run binds the public
// port. A deployment can therefore reuse persistent LKG state through a
// dependency outage, but can never become Live with an empty/degraded model.
func (a *app) ensureMaterializedReadModelsReady(ctx context.Context) error {
	deadline := time.NewTicker(750 * time.Millisecond)
	defer deadline.Stop()
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := a.ensureDashboardReadModelReady(ctx); err != nil {
			lastErr = fmt.Errorf("dashboard: %w", err)
		} else if err := a.ensureCentralReadModelsReady(ctx); err != nil {
			lastErr = err
		} else if err := a.ensurePartnerReadModelsReady(ctx); err != nil {
			lastErr = err
		} else {
			return nil
		}
		if a.log != nil {
			a.log.Warn("materialized read-model readiness attempt incomplete", "attempt", attempt, "error", lastErr)
		}
		if attempt == 3 {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("read-model readiness deadline: %w", ctx.Err())
		case <-deadline.C:
		}
	}
	return lastErr
}


// verifyColdStartLKG closes the cold-start race explicitly: every required
// persistent Central key is re-read from PostgreSQL, validated as healthy LKG,
// and mirrored in memory before readiness can become true.
func (a *app) verifyColdStartLKG(ctx context.Context) error {
	for key := range centralReadModelBaselines() {
		payload, updated, err := a.loadCentralSnapshotDB(ctx, key)
		if err != nil {
			return fmt.Errorf("cold-start LKG %s: %w", key, err)
		}
		if !centralSnapshotValid(key, payload) {
			return fmt.Errorf("cold-start LKG %s is invalid", key)
		}
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated.UTC(),
		}
		centralStep3Snapshots.Unlock()
	}

	year := time.Now().UTC().Year()
	dashboard, _, err := a.loadDashboardSnapshotContext(ctx, year)
	if err != nil {
		return fmt.Errorf("cold-start dashboard LKG: %w", err)
	}
	if !dashboardSnapshotValid(dashboard) {
		return fmt.Errorf("cold-start dashboard LKG is invalid")
	}

	partners, _, err := a.loadCentralSnapshotDB(ctx, centralStep4PartnersKey)
	if err != nil {
		return fmt.Errorf("cold-start partner registry LKG: %w", err)
	}
	for _, item := range step4Items(partners["items"]) {
		partnerID := strings.TrimSpace(central10String(item["id"]))
		if partnerID == "" {
			continue
		}
		payload, updated, err := a.loadPartnerWorkspaceDB(ctx, partnerID)
		if err != nil {
			return fmt.Errorf("cold-start tenant LKG %s: %w", partnerID, err)
		}
		key := centralPartnerWorkspaceKey(partnerID)
		centralStep3Snapshots.Lock()
		centralStep3Snapshots.items[key] = centralStep3SnapshotEntry{
			payload: centralStep3CopyMap(payload), updatedAt: updated.UTC(),
		}
		centralStep3Snapshots.Unlock()
	}
	return nil
}

// ensureColdStartReadiness is the two-phase startup contract:
//   1. persist deterministic healthy baselines for every missing/corrupt key;
//   2. rebuild what is available, validate DB+memory LKG, then atomically open
//      the request gate. No public listener is bound before this returns.
func (a *app) ensureColdStartReadiness(ctx context.Context) error {
	gatewayReadiness.Store(false)

	if err := a.seedCentralReadModelBaselines(ctx); err != nil {
		return fmt.Errorf("seed deterministic read-model baselines: %w", err)
	}

	a.bootstrapDashboardSnapshot()
	a.bootstrapCentralStep3Snapshots()
	a.bootstrapPartnerWorkspaceSnapshots()

	// Prefer authoritative projections when dependencies are already ready, but
	// never replace a valid LKG baseline with unavailable/partial state.
	a.warmMissingCentralSnapshots()
	a.warmMissingCentralPartnerWorkspaces()
	a.processReadModelRefreshQueue()

	if err := a.ensureMaterializedReadModelsReady(ctx); err != nil {
		return err
	}
	if err := a.ensureCentralUserNotificationReadModelsReady(ctx); err != nil {
		return fmt.Errorf("Central user read models not ready: %w", err)
	}
	if err := a.verifyColdStartLKG(ctx); err != nil {
		return err
	}
	// CENTRAL-10..21 cold-start contract: exact screen payloads are encoded
	// into immutable byte slices before readiness opens. The first read-model
	// hit therefore performs no DB lookup, JSON decode/encode, or downstream I/O.
	if err := a.prewarmCentral10To21HotResponses(ctx); err != nil {
		return fmt.Errorf("serialized Central/Tenant prewarm: %w", err)
	}

	gatewayReadiness.Store(true)
	return nil
}
