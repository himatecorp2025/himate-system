package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const readModelSeedVersion = 1

func readModelSeeded(payload map[string]any) bool {
	return payload != nil && payload["seeded"] == true
}

func emptyPage(limit int) map[string]any {
	return map[string]any{
		"items": []map[string]any{},
		"count": 0,
		"total": 0,
		"limit": limit,
		"offset": 0,
		"has_more": false,
	}
}

func centralReadModelBaselines() map[string]map[string]any {
	base := func(payload map[string]any) map[string]any {
		payload["status"] = "healthy"
		payload["unavailable"] = []string{}
		payload["seeded"] = true
		payload["seed_version"] = readModelSeedVersion
		return payload
	}
	return map[string]map[string]any{
		centralStep3RegistryKey: base(map[string]any{
			"modules": []map[string]any{},
			"groups": []map[string]any{},
			"trend": []map[string]any{},
			"module_details": map[string]any{},
		}),
		centralStep3PlansKey: base(map[string]any{
			"plans": []map[string]any{},
			"modules": []map[string]any{},
		}),
		centralStep3AnalyticsKey: base(map[string]any{
			"analytics": map[string]any{"packages": []map[string]any{}},
			"modules": []map[string]any{},
		}),
		centralStep3CommercialKey: base(map[string]any{
			"partners": []map[string]any{},
			"matrix_items": []map[string]any{},
			"subscription_items": []map[string]any{},
			"modules": []map[string]any{},
			"plans": []map[string]any{},
			"matrix_available": true,
			"subscriptions_available": true,
		}),
		centralStep4PartnersKey: base(map[string]any{
			"items": []map[string]any{},
			"categories_raw": []map[string]any{},
			"pagination": map[string]any{
				"count": 0, "total": 0, "limit": 24, "offset": 0, "has_more": false,
			},
			"kpis": map[string]any{
				"partner_records": 0,
				"live_partners": 0,
				"prospects": 0,
				"reference_partners": 0,
				"lifecycle_counts": map[string]int{},
			},
		}),
		centralStep4FinanceKey: base(map[string]any{
			"profile": map[string]any{},
			"overview": map[string]any{
				"currencies": []map[string]any{},
				"onboarding": map[string]any{"items": []map[string]any{}, "pending": 0},
				"monthly_paid": []map[string]any{},
				"weekly_paid": []map[string]any{},
				"monthly_paid_by_plan": []map[string]any{},
				"weekly_paid_by_plan": []map[string]any{},
			},
			"invoices": []map[string]any{},
			"partners": []map[string]any{},
		}),
		centralStep4ImpactKey: base(map[string]any{
			"definitions": []map[string]any{},
			"summary": []map[string]any{},
			"evidence": []map[string]any{},
			"evidence_integrity": map[string]any{},
			"reports": []map[string]any{},
			"analytics": map[string]any{},
		}),
		centralStep4AdministrationKey: base(map[string]any{
			"company": map[string]any{"profile": map[string]any{}},
			"items": []map[string]any{},
			"kpis": map[string]any{},
			"admin_roles": emptyPage(100),
			"admin_users": emptyPage(100),
			"admin_secrets": emptyPage(100),
			"audit_events": emptyPage(100),
			"company_documents": emptyPage(100),
			"invoice_register": emptyPage(100),
			"backup_api": map[string]any{"provider": "", "items": []map[string]any{}},
		}),
		centralStep4SystemKey: base(map[string]any{
			"health": map[string]any{"status": "OK", "services": []map[string]any{}, "partners": []map[string]any{}},
			"provisioning": []map[string]any{},
			"environments": []map[string]any{},
			"events": []map[string]any{},
			"backups": map[string]any{"provider": "", "items": []map[string]any{}},
			"kpis": map[string]any{
				"healthy_services": 0,
				"service_count": 0,
				"partner_systems": 0,
				"deployed_environments": 0,
				"environment_count": 0,
				"issues": 0,
				"degraded_partners": 0,
			},
			"health_api": map[string]any{
				"status": "OK",
				"services": []map[string]any{},
				"partners": []map[string]any{},
				"summary": map[string]any{
					"services": 0, "partners": 0, "partner_errors": 0, "partner_degraded": 0,
				},
			},
			"provisioning_api": emptyPage(100),
			"environments_api": emptyPage(100),
			"backups_api": map[string]any{"provider": "", "items": []map[string]any{}},
			"backup_restore_points": map[string]any{},
			"backup_restore_tests_api": emptyPage(100),
			"backup_restore_jobs_api": emptyPage(100),
		}),
		centralStep4WebsiteKey: base(map[string]any{
			"pages": []map[string]any{},
			"media": []map[string]any{},
			"environments": []map[string]any{},
			"kpis": map[string]any{
				"pages": 0, "published_pages": 0, "media_assets": 0,
				"image_assets": 0, "environments": 0, "live_environments": 0,
			},
			"seo": map[string]any{},
			"seo_audit": map[string]any{},
			"contact_inquiries": emptyPage(200),
			"cms_design": map[string]any{},
			"cms_page_details": map[string]any{},
			"cms_page_versions": map[string]any{},
			"cms_page_audits": map[string]any{},
		}),
		centralStep4ConnectionsKey: base(map[string]any{
			"items": []map[string]any{},
			"kpis": map[string]any{
				"partner_count": 0, "active": 0, "inactive": 0,
				"suspended": 0, "deleted": 0, "integration_count": 0,
			},
			"start22_mapping": map[string]any{},
			"start22_summary": map[string]any{
				"ALL": map[string]any{},
				"PRODUCTION": map[string]any{},
				"STAGING": map[string]any{},
			},
		}),
		centralStep4ComplianceKey: base(map[string]any{
			"items": []map[string]any{},
			"details": map[string]any{},
		}),
		centralStep4GlobalSearchKey: base(map[string]any{
			"partners": []map[string]any{},
			"modules": []map[string]any{},
			"contact_inquiries": []map[string]any{},
			"cms_pages": []map[string]any{},
			"admin_users": []map[string]any{},
			"audit_events": []map[string]any{},
		}),
	}
}

func dashboardReadModelBaseline(year int, env, version string) map[string]any {
	block := func(extra map[string]any) map[string]any {
		out := map[string]any{"available": true, "status": "healthy"}
		for key, value := range extra {
			out[key] = value
		}
		return out
	}
	return map[string]any{
		"year": year,
		"seeded": true,
		"seed_version": readModelSeedVersion,
		"partners": block(map[string]any{
			"items": []map[string]any{}, "count": 0, "live": 0, "prospects": 0,
			"reference_partners": 0, "trend": []map[string]any{},
		}),
		"modules": block(map[string]any{
			"items": []map[string]any{}, "count": 0, "active": 0, "in_development": 0,
		}),
		"billing": block(map[string]any{
			"currencies": []map[string]any{}, "monthly_paid": []map[string]any{},
			"outstanding": []map[string]any{},
		}),
		"impact": block(map[string]any{
			"definitions": []map[string]any{}, "summary": []map[string]any{},
		}),
		"partner_geo": block(map[string]any{
			"country": "US", "active_states": 0, "active_partners": 0,
			"states": []map[string]any{}, "partners": []map[string]any{},
		}),
		"activity": map[string]any{
			"items": []map[string]any{}, "count": 0,
			"source": "IDENTITY_APPEND_ONLY_AUDIT", "status": "healthy",
		},
		"system": map[string]any{
			"status": "healthy", "environment": env, "version": version,
			"architecture": "materialized-dashboard-snapshot",
		},
		"meta": map[string]any{
			"architecture": "MATERIALIZED_DASHBOARD_SNAPSHOT",
			"status": "healthy", "unavailable": []string{},
			"generated_at": time.Now().UTC(), "seeded": true,
			"refresh_interval_ms": dashboardSnapshotRefreshInterval.Milliseconds(),
		},
	}
}

func partnerWorkspaceBaseline(partner map[string]any) map[string]any {
	partnerID := central10String(partner["id"])
	return map[string]any{
		"status": "healthy",
		"unavailable": []string{},
		"seeded": true,
		"seed_version": readModelSeedVersion,
		"partner": central10CopyMap(partner),
		"modules": []map[string]any{},
		"module_view": central10PartnerModuleView(nil, nil, "", "ALL"),
		"production_environment": nil,
		"preferred_connector_environment": "STAGING",
		"billing": map[string]any{},
		"company_profile": map[string]any{},
		"terms": map[string]any{},
		"license": map[string]any{},
		"documents": []map[string]any{},
		"invoices": []map[string]any{},
		"subscriptions": []map[string]any{},
		"environments": []map[string]any{},
		"provisioning_jobs": []map[string]any{},
		"impact_summary": []map[string]any{},
		"evidence": []map[string]any{},
		"connector_credentials": []map[string]any{},
		"portal_users": []map[string]any{},
		"agreement": map[string]any{},
		"commercial_status": map[string]any{},
		"billing_events": []map[string]any{},
		"website_adapter": map[string]any{},
		"partner_design": map[string]any{},
		"payment_profile": map[string]any{},
		"catalog_modules_api": emptyPage(100),
		"environments_api": emptyPage(100),
		"provisioning_api": emptyPage(100),
		"impact_api": emptyPage(100),
		"evidence_api": emptyPage(100),
		"connector_credentials_api": emptyPage(100),
		"portal_users_api": emptyPage(100),
		"portal_gate": map[string]any{
			"partner_id": partnerID, "allowed": false, "reason": "BASELINE_REQUIRES_MATERIALIZATION",
		},
		"portal_modules": emptyPage(100),
		"portal_plans": emptyPage(100),
		"portal_plan": map[string]any{},
		"portal_plan_modules": emptyPage(100),
		"portal_charity": map[string]any{},
		"portal_charity_modules": emptyPage(100),
		"portal_design_media": emptyPage(100),
		"portal_billing_subscriptions": emptyPage(100),
		"portal_billing_invoices": emptyPage(100),
		"portal_user_module_policies": emptyPage(100),
		"portal_notifications": emptyPage(100),
		"portal_impact": emptyPage(100),
		"tenant_finance": map[string]any{},
		"partner_audit_events": emptyPage(100),
		"partner_contacts": emptyPage(100),
		"partner_domains_deployments": map[string]any{
			"domains": []map[string]any{}, "deployments": []map[string]any{},
		},
		"partner_permissions": emptyPage(100),
		"module_commercial_history": map[string]any{},
		"start22_summary": map[string]any{},
		"start22_retention": map[string]any{},
	}
}

func (a *app) seedCentralReadModelBaselines(ctx context.Context) error {
	for key, payload := range centralReadModelBaselines() {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal baseline %s: %w", key, err)
		}
		if _, err := a.db.ExecContext(ctx,
			`INSERT INTO identity.central_screen_snapshots(snapshot_key,payload,updated_at)
			 VALUES($1,$2::jsonb,NOW())
			 ON CONFLICT(snapshot_key) DO NOTHING`,
			key, string(raw),
		); err != nil {
			return fmt.Errorf("seed baseline %s: %w", key, err)
		}
	}

	year := time.Now().UTC().Year()
	dashboard := dashboardReadModelBaseline(year, a.env, a.version)
	raw, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("marshal dashboard baseline: %w", err)
	}
	if _, err := a.db.ExecContext(ctx,
		`INSERT INTO identity.dashboard_snapshots(year,payload,updated_at)
		 VALUES($1,$2::jsonb,NOW())
		 ON CONFLICT(year) DO NOTHING`,
		year, string(raw),
	); err != nil {
		return fmt.Errorf("seed dashboard baseline: %w", err)
	}
	return a.seedPartnerBaselinesFromCentralSnapshot(ctx)
}

func (a *app) seedPartnerWorkspaceBaseline(ctx context.Context, partner map[string]any) error {
	partnerID := strings.TrimSpace(central10String(partner["id"]))
	if partnerID == "" {
		return nil
	}
	payload := partnerWorkspaceBaseline(partner)
	if !partnerWorkspaceSnapshotValid(payload) {
		return fmt.Errorf("partner baseline failed validation: %s", partnerID)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx,
		`INSERT INTO identity.partner_workspace_snapshots(partner_id,payload,updated_at)
		 VALUES($1,$2::jsonb,NOW())
		 ON CONFLICT(partner_id) DO NOTHING`,
		partnerID, string(raw),
	)
	return err
}

func (a *app) seedPartnerBaselinesFromCentralSnapshot(ctx context.Context) error {
	var raw []byte
	if err := a.db.QueryRowContext(ctx,
		`SELECT payload FROM identity.central_screen_snapshots WHERE snapshot_key=$1`,
		centralStep4PartnersKey,
	).Scan(&raw); err != nil {
		return err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	for _, partner := range step4Items(payload["items"]) {
		if err := a.seedPartnerWorkspaceBaseline(ctx, partner); err != nil {
			return err
		}
	}
	return nil
}
