package main

import (
	"context"

	"himate.local/services/internal/common"
)

const selectionModeUnlimited = "UNLIMITED"

type billingTaxPolicy struct {
	RatePercent  float64
	Label        string
	Jurisdiction string
}

func central5BillingMigration() common.Migration {
	return common.Migration{
		Version:                17,
		Name:                   "central-5-packages-pricing-vat-unlimited",
		AllowDestructiveSchema: true,
		Statements: []string{
			`ALTER TABLE billing.subscription_plans DROP CONSTRAINT IF EXISTS subscription_plans_selection_mode_check`,
			`ALTER TABLE billing.subscription_plans ADD CONSTRAINT subscription_plans_selection_mode_check
				CHECK(selection_mode IN ('FIXED','SELECTABLE','CUSTOM','UNLIMITED'))`,
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS vat_rate_percent NUMERIC(6,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS vat_jurisdiction TEXT NOT NULL DEFAULT 'GB'`,
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS tax_label TEXT NOT NULL DEFAULT 'VAT'`,
			`ALTER TABLE billing.company_profile DROP CONSTRAINT IF EXISTS billing_company_profile_vat_rate_check`,
			`ALTER TABLE billing.company_profile ADD CONSTRAINT billing_company_profile_vat_rate_check
				CHECK(vat_rate_percent>=0 AND vat_rate_percent<=100)`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS net_total NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS tax_rate_percent NUMERIC(6,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`UPDATE billing.invoices SET net_total=total WHERE net_total=0 AND total<>0`,
			`UPDATE billing.subscription_plans SET
				display_name='Starter',monthly_price=990,annual_list_price=11880,annual_price=11880,
				module_limit=10,selection_mode='FIXED',customer_selectable=TRUE,sort_order=10,updated_at=NOW()
				WHERE plan_key='STARTER'`,
			`UPDATE billing.subscription_plans SET
				display_name='Business',monthly_price=1490,annual_list_price=17880,annual_price=16390,
				module_limit=20,selection_mode='FIXED',customer_selectable=TRUE,sort_order=20,updated_at=NOW()
				WHERE plan_key='BUSINESS'`,
			`UPDATE billing.subscription_plans SET
				display_name='Premium',monthly_price=2490,annual_list_price=29880,annual_price=22410,
				module_limit=0,selection_mode='UNLIMITED',customer_selectable=TRUE,sort_order=30,updated_at=NOW()
				WHERE plan_key='FLEX'`,
			`INSERT INTO billing.subscription_plan_price_history(
				plan_key,currency,monthly_price,annual_list_price,annual_price,effective_from,change_type,
				annual_increase_percent,actor,reason)
				SELECT p.plan_key,p.currency,p.monthly_price,p.annual_list_price,p.annual_price,CURRENT_DATE,
					'CENTRAL5_BASELINE',p.annual_increase_percent,'migration',
					'Central-5 10/20/Unlimited package and pricing baseline'
				FROM billing.subscription_plans p
				WHERE p.plan_key IN ('STARTER','BUSINESS','FLEX')
				  AND NOT EXISTS(
					SELECT 1 FROM billing.subscription_plan_price_history h
					WHERE h.plan_key=p.plan_key AND h.change_type='CENTRAL5_BASELINE'
				  )`,
			`CREATE OR REPLACE FUNCTION billing.guard_plan_invoice_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' THEN
					IF OLD.billing_model='PLAN' THEN
						RAISE EXCEPTION 'PLAN invoices are append-only';
					END IF;
					RETURN OLD;
				END IF;
				IF OLD.billing_model IS DISTINCT FROM NEW.billing_model THEN
					RAISE EXCEPTION 'invoice billing_model is immutable';
				END IF;
				IF OLD.billing_model='PLAN' AND (
					OLD.invoice_key IS DISTINCT FROM NEW.invoice_key
					OR OLD.partner_id IS DISTINCT FROM NEW.partner_id
					OR OLD.invoice_date IS DISTINCT FROM NEW.invoice_date
					OR OLD.service_period_start IS DISTINCT FROM NEW.service_period_start
					OR OLD.service_period_end IS DISTINCT FROM NEW.service_period_end
					OR OLD.currency IS DISTINCT FROM NEW.currency
					OR OLD.base_fee IS DISTINCT FROM NEW.base_fee
					OR OLD.module_fee IS DISTINCT FROM NEW.module_fee
					OR OLD.total IS DISTINCT FROM NEW.total
					OR OLD.net_total IS DISTINCT FROM NEW.net_total
					OR OLD.tax_rate_percent IS DISTINCT FROM NEW.tax_rate_percent
					OR OLD.tax_amount IS DISTINCT FROM NEW.tax_amount
					OR OLD.minimum_commitment_adjustment IS DISTINCT FROM NEW.minimum_commitment_adjustment
					OR OLD.plan_key IS DISTINCT FROM NEW.plan_key
					OR OLD.billing_frequency IS DISTINCT FROM NEW.billing_frequency
					OR OLD.charge_type IS DISTINCT FROM NEW.charge_type
					OR OLD.list_price IS DISTINCT FROM NEW.list_price
					OR OLD.discount_amount IS DISTINCT FROM NEW.discount_amount
					OR OLD.created_at IS DISTINCT FROM NEW.created_at
				) THEN
					RAISE EXCEPTION 'PLAN invoice commercial fields are immutable';
				END IF;
				RETURN NEW;
			END; $fn$`,
		},
	}
}

func (a *app) loadBillingTaxPolicy(ctx context.Context) (billingTaxPolicy, error) {
	p := billingTaxPolicy{Label: "VAT", Jurisdiction: "GB"}
	err := a.db.QueryRowContext(ctx, `SELECT vat_rate_percent,tax_label,vat_jurisdiction
		FROM billing.company_profile WHERE id=1`).Scan(&p.RatePercent, &p.Label, &p.Jurisdiction)
	return p, err
}

func applyBillingTax(net float64, policy billingTaxPolicy) (float64, float64, float64) {
	net = roundPlanAmount(net)
	rate := policy.RatePercent
	if rate < 0 {
		rate = 0
	}
	tax := roundPlanAmount(net * rate / 100)
	return net, tax, roundPlanAmount(net + tax)
}

func addTaxQuote(out map[string]any, monthly, annual float64, policy billingTaxPolicy) {
	monthlyNet, monthlyTax, monthlyGross := applyBillingTax(monthly, policy)
	annualNet, annualTax, annualGross := applyBillingTax(annual, policy)
	out["price_basis"] = "NET_PLUS_TAX"
	out["tax_label"] = policy.Label
	out["vat_rate_percent"] = policy.RatePercent
	out["vat_jurisdiction"] = policy.Jurisdiction
	out["vat_enabled"] = policy.RatePercent > 0
	out["monthly_net_price"] = monthlyNet
	out["monthly_tax_amount"] = monthlyTax
	out["monthly_gross_price"] = monthlyGross
	out["annual_net_price"] = annualNet
	out["annual_tax_amount"] = annualTax
	out["annual_gross_price"] = annualGross
}

func (a *app) activePlanPartnerCount(ctx context.Context, planKey string) (int, error) {
	var count int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing.partner_plan_subscriptions
		WHERE plan_key=$1 AND status='ACTIVE'`, planKey).Scan(&count)
	return count, err
}

func (a *app) decoratePackageMap(ctx context.Context, out map[string]any, p subscriptionPlan) error {
	policy, err := a.loadBillingTaxPolicy(ctx)
	if err != nil {
		return err
	}
	addTaxQuote(out, p.MonthlyPrice, p.AnnualPrice, policy)
	count, err := a.activePlanPartnerCount(ctx, p.Key)
	if err != nil {
		return err
	}
	out["active_partner_count"] = count
	out["entitlement_mode"] = p.SelectionMode
	out["unlimited_modules"] = p.SelectionMode == selectionModeUnlimited
	return nil
}
