package main

import "himate.local/services/internal/common"

// central18BillingPackageRecoveryMigration repairs only the legacy package
// baseline that existed before CENTRAL-5/8. Explicit administrator pricing is
// preserved: a latest MANUAL price-history row is never overwritten.
func central18BillingPackageRecoveryMigration() common.Migration {
	return common.Migration{
		Version: 21,
		Name:    "central-18-canonical-package-baseline-recovery",
		Statements: []string{
			`WITH latest AS (
				SELECT DISTINCT ON (plan_key)
					plan_key,monthly_price,annual_list_price,annual_price,change_type
				FROM billing.subscription_plan_price_history
				WHERE effective_from<=CURRENT_DATE
				ORDER BY plan_key,effective_from DESC,
					CASE change_type WHEN 'MANUAL' THEN 3 WHEN 'ANNUAL_INCREASE' THEN 2 ELSE 1 END DESC,
					id DESC
			), recovery(plan_key,legacy_monthly,monthly,list_price,annual_price,module_limit,selection_mode,display_name) AS (
				VALUES
					('STARTER',500::numeric,990::numeric,11880::numeric,11880::numeric,10,'FIXED','Starter'),
					('BUSINESS',1500::numeric,1490::numeric,17880::numeric,16390::numeric,20,'FIXED','Business'),
					('FLEX',2500::numeric,2490::numeric,29880::numeric,22410::numeric,0,'UNLIMITED','Premium')
			)
			INSERT INTO billing.subscription_plan_price_history(
				plan_key,currency,monthly_price,annual_list_price,annual_price,effective_from,change_type,
				annual_increase_percent,actor,reason
			)
			SELECT p.plan_key,p.currency,r.monthly,r.list_price,r.annual_price,CURRENT_DATE,'CENTRAL18_RECOVERY',
				p.annual_increase_percent,'migration','CENTRAL-18 legacy package baseline recovery'
			FROM billing.subscription_plans p
			JOIN recovery r ON r.plan_key=p.plan_key
			LEFT JOIN latest l ON l.plan_key=p.plan_key
			WHERE (p.monthly_price=r.legacy_monthly OR (l.monthly_price=r.legacy_monthly AND COALESCE(l.change_type,'')<>'MANUAL'))
			  AND NOT EXISTS (
				SELECT 1 FROM billing.subscription_plan_price_history h
				WHERE h.plan_key=p.plan_key AND h.change_type='CENTRAL18_RECOVERY'
			  )`,
			`UPDATE billing.subscription_plans SET
				display_name='Starter',
				monthly_price=CASE WHEN monthly_price=500 THEN 990 ELSE monthly_price END,
				annual_list_price=CASE WHEN monthly_price=500 THEN 11880 ELSE annual_list_price END,
				annual_price=CASE WHEN monthly_price=500 THEN 11880 ELSE annual_price END,
				module_limit=10,selection_mode='FIXED',customer_selectable=TRUE,sort_order=10,updated_at=NOW()
				WHERE plan_key='STARTER'`,
			`UPDATE billing.subscription_plans SET
				display_name='Business',
				monthly_price=CASE WHEN monthly_price=1500 THEN 1490 ELSE monthly_price END,
				annual_list_price=CASE WHEN monthly_price=1500 THEN 17880 ELSE annual_list_price END,
				annual_price=CASE WHEN monthly_price=1500 THEN 16390 ELSE annual_price END,
				module_limit=20,selection_mode='FIXED',customer_selectable=TRUE,sort_order=20,updated_at=NOW()
				WHERE plan_key='BUSINESS'`,
			`UPDATE billing.subscription_plans SET
				display_name='Premium',
				monthly_price=CASE WHEN monthly_price=2500 THEN 2490 ELSE monthly_price END,
				annual_list_price=CASE WHEN monthly_price=2500 THEN 29880 ELSE annual_list_price END,
				annual_price=CASE WHEN monthly_price=2500 THEN 22410 ELSE annual_price END,
				module_limit=0,selection_mode='UNLIMITED',customer_selectable=TRUE,sort_order=30,updated_at=NOW()
				WHERE plan_key='FLEX'`,
		},
	}
}
