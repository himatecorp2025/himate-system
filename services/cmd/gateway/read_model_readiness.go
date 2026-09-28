package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type centralReadinessJob struct {
	key     string
	refresh func()
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
