#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
billing_main = (root / 'services/cmd/billing/main.go').read_text()
billing_plans = (root / 'services/cmd/billing/plans.go').read_text()
catalog_main = (root / 'services/cmd/catalog/main.go').read_text()
catalog_plans = (root / 'services/cmd/catalog/plans.go').read_text()
gateway = (root / 'services/cmd/gateway/partner_portal.go').read_text()
module_ui = (root / 'frontend/lib/module_control_plane.dart').read_text()
portal_ui = (root / 'frontend/lib/partner_portal.dart').read_text()
openapi = (root / 'docs/openapi.yaml').read_text()
acceptance = (root / 'docs/START-23.11.2_ACCEPTANCE.md').read_text()

def require(condition, message):
    if not condition:
        raise SystemExit('START-23.11.2 audit failed: ' + message)

for token in [
    "start23112PlanBillingMigration()",
    "'STARTER','Starter','USD',500,6000,6000,0,3,'FIXED'",
    "'BUSINESS','Business','USD',1500,18000,16500,1,10,'FIXED'",
    "'FLEX','Flex','USD',2500,30000,22500,3,15,'SELECTABLE'",
    "partner_plan_subscriptions",
    "partner_plan_module_selections",
    "plan_change_history",
    "PLAN_BASED",
    "PLAN_UPGRADE",
    "PLAN_ANNUAL_PREPAY",
    "PLAN_MONTHLY",
]:
    require(token in billing_plans or token in billing_main, 'missing plan billing token: ' + token)

require('start23112CalendarMonthBillingMigration' not in billing_main, 'obsolete calendar-month module migration is still wired')
require('runPlanBillingCycle(ctx, at)' in billing_main, 'plan billing cycle is not authoritative in invoice runner')
require('if planManaged[id] { continue }' in billing_main, 'legacy module invoice path is not bypassed for plan partners')
require('ACTIVATION_LICENSE_REQUIRED' in billing_plans, 'activation-license gate is missing')
require('targetPrice-currentPrice' in billing_plans, 'immediate full plan-price upgrade difference is missing')
require('SCHEDULED_DOWNGRADE' in billing_plans, 'scheduled downgrade state is missing')
require('nextMonthStart(time.Now().UTC())' in billing_plans, 'next-month boundary scheduling is missing')
require("billing_model':'PLAN'" in billing_plans.replace(' ', '') or "'PLAN'" in billing_plans, 'PLAN ledger marker is missing')

for token in [
    'start23112CatalogPlanMigration()',
    'entitlement_source',
    'plan-entitlements',
    'applyPlanEntitlements',
]:
    require(token in catalog_main or token in catalog_plans, 'missing Catalog plan entitlement token: ' + token)

for token in [
    '/partner/api/v1/plans',
    '/partner/api/v1/plan',
    'PLAN_MANAGED_MODULES',
]:
    require(token in gateway, 'missing Partner Portal plan boundary: ' + token)

for token in [
    'Subscription Plans',
    'Starter: $500/month',
    'Business: $1,500/month',
    'Flex: $2,500/month',
    'annual_list_price',
    'annual_price',
    'TextDecoration.lineThrough',
]:
    require(token in module_ui or token in portal_ui, 'missing plan UI token: ' + token)

require('18,000' in acceptance and '16,500' in acceptance, 'Business annual full/discounted amounts are not documented')
require('30,000' in acceptance and '22,500' in acceptance, 'Flex annual full/discounted amounts are not documented')
require('module recurring charge' in acceptance.lower(), 'acceptance does not state that modules are not recurring invoice authority')

for token in [
    '/api/v1/billing/plans:',
    '/api/v1/billing/plans/{planKey}:',
    '/api/v1/billing/partners/{partnerId}/plan:',
    '/api/v1/billing/partners/{partnerId}/plan/modules:',
]:
    require(token in openapi, 'OpenAPI missing plan route: ' + token)

print('HIMATE START-23.11.2 subscription-plan billing audit passed')
