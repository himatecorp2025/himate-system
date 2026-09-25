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
		Version: 17,
		Name:    "central-5-packages-pricing-vat-unlimited",
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
