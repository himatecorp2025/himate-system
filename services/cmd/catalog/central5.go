package main

import (
	"database/sql"
	"strings"

	"himate.local/services/internal/common"
)

const catalogEntitlementModeUnlimited = "UNLIMITED"

func central5CatalogMigration() common.Migration {
	return common.Migration{
		Version: 11,
		Name:    "central-5-dynamic-package-entitlement-policy",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS catalog.partner_plan_entitlement_policies(
				partner_id TEXT PRIMARY KEY,
				plan_key TEXT NOT NULL DEFAULT '',
				entitlement_mode TEXT NOT NULL DEFAULT 'FIXED',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(entitlement_mode IN ('FIXED','UNLIMITED'))
			)`,
			`CREATE INDEX IF NOT EXISTS partner_plan_entitlement_policy_mode_idx
				ON catalog.partner_plan_entitlement_policies(entitlement_mode,plan_key,partner_id)`,
		},
	}
}

func (a *app) ensureDynamicPlanEntitlements(partnerID string) error {
	var planKey, mode string
	err := a.db.QueryRow(`SELECT plan_key,entitlement_mode FROM catalog.partner_plan_entitlement_policies
		WHERE partner_id=$1`, partnerID).Scan(&planKey, &mode)
	if err == sql.ErrNoRows {
		var legacyPremium bool
		if scanErr := a.db.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM catalog.partner_modules
			WHERE partner_id=$1 AND entitlement_source='PLAN' AND plan_key='FLEX'
		)`, partnerID).Scan(&legacyPremium); scanErr != nil {
			return scanErr
		}
		if !legacyPremium {
			return nil
		}
		planKey = "FLEX"
		mode = catalogEntitlementModeUnlimited
		if _, upsertErr := a.db.Exec(`INSERT INTO catalog.partner_plan_entitlement_policies(partner_id,plan_key,entitlement_mode)
			VALUES($1,$2,$3)
			ON CONFLICT(partner_id) DO UPDATE SET plan_key=EXCLUDED.plan_key,entitlement_mode=EXCLUDED.entitlement_mode,updated_at=NOW()`,
			partnerID, planKey, mode); upsertErr != nil {
			return upsertErr
		}
	} else if err != nil {
		return err
	}
	if strings.ToUpper(strings.TrimSpace(mode)) != catalogEntitlementModeUnlimited {
		return nil
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`UPDATE catalog.partner_modules pm SET
		status='NOT_LICENSED',entitlement_state='INACTIVE',visible=FALSE,included_in_base=FALSE,
		plan_effective_at=NOW(),updated_at=NOW()
		FROM catalog.modules m
		WHERE pm.partner_id=$1 AND pm.module_key=m.module_key
		  AND pm.entitlement_source='PLAN' AND pm.plan_key=$2
		  AND NOT (m.publication_status='PUBLISHED' AND m.implementation_state='READY' AND m.availability='ACTIVE')`,
		partnerID, planKey); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE catalog.partner_modules pm SET
		status='ACTIVE',entitlement_state='ACTIVE',visible=TRUE,included_in_base=TRUE,
		commercial_configured=TRUE,
		contract_currency=CASE WHEN pm.contract_currency='' THEN 'USD' ELSE pm.contract_currency END,
		quote_reference='PLAN:'||$2,entitlement_source='PLAN',plan_key=$2,
		plan_effective_at=NOW(),activated_at=COALESCE(pm.activated_at,NOW()),updated_at=NOW()
		FROM catalog.modules m
		WHERE pm.partner_id=$1 AND pm.module_key=m.module_key
		  AND m.publication_status='PUBLISHED' AND m.implementation_state='READY' AND m.availability='ACTIVE'`,
		partnerID, planKey); err != nil {
		return err
	}
	return tx.Commit()
}
