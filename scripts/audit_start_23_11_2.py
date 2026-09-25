#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
billing_main = (root / 'services/cmd/billing/main.go').read_text()
billing_plans = (root / 'services/cmd/billing/plans.go').read_text()
billing_dunning = (root / 'services/cmd/billing/dunning.go').read_text()
central5_billing = (root / 'services/cmd/billing/central5.go').read_text()
central5_catalog = (root / 'services/cmd/catalog/central5.go').read_text()
partners_main = (root / 'services/cmd/partners/main.go').read_text()
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
    "central5BillingMigration()",
    "display_name='Starter',monthly_price=990",
    "display_name='Business',monthly_price=1490",
    "display_name='Premium',monthly_price=2490",
    "selection_mode='UNLIMITED'",
    "vat_rate_percent",
    "partner_plan_subscriptions",
    "partner_plan_module_selections",
    "plan_change_history",
    "PLAN_BASED",
    "PLAN_UPGRADE",
    "PLAN_ANNUAL_PREPAY",
    "PLAN_MONTHLY",
]:
    require(token in billing_plans or token in billing_main or token in central5_billing, 'missing plan billing token: ' + token)

require('start23112CalendarMonthBillingMigration' not in billing_main, 'obsolete calendar-month module migration is still wired')
require('runPlanBillingCycle(ctx, at)' in billing_main, 'plan billing cycle is not authoritative in invoice runner')
require('if planManaged[id] { continue }' in billing_main, 'legacy module invoice path is not bypassed for plan partners')
require('ACTIVATION_LICENSE_REQUIRED' in billing_plans, 'activation-license gate is missing')
require('targetPrice-currentPrice' in billing_plans, 'immediate full plan-price upgrade difference is missing')
require('SCHEDULED_DOWNGRADE' in billing_plans, 'scheduled downgrade state is missing')
require('nextMonthStart(time.Now().UTC())' in billing_plans, 'next-month boundary scheduling is missing')
require("billing_model':'PLAN'" in billing_plans.replace(' ', '') or "'PLAN'" in billing_plans, 'PLAN ledger marker is missing')

for token in [
    'start23112DunningMigration()',
    'dunningMaxAttempts = 3',
    'dunningSecondOffset = 2',
    'dunningThirdOffset  = 5',
    'dunningCureDays     = 30',
    'PAYMENT_RETRY_SCHEDULED',
    'PARTNER_SUSPENDED_NONPAYMENT',
    'PARTNER_OPERATIONAL_ACCOUNT_PURGED',
    'recoverDunningPayment',
    'purgePartnerOperationalAccess',
]:
    require(token in billing_dunning, 'missing dunning lifecycle token: ' + token)

for token in [
    'purge-operational',
    'identity.partner_users',
    "lifecycle='ARCHIVED'",
]:
    require(token in partners_main, 'missing retention-safe operational purge token: ' + token)


for token in [
    'start23112CatalogPlanMigration()',
    'entitlement_source',
    'plan-entitlements',
    'applyPlanEntitlements',
]:
    require(token in catalog_main or token in catalog_plans, 'missing Catalog plan entitlement token: ' + token)

for token in [
    'case path=="/plans"',
    'case path=="/plan"',
    'case path=="/plan/modules"',
    'partnerPlans(w,r,u)',
    'PLAN_MANAGED_MODULES',
]:
    require(token in gateway, 'missing Partner Portal plan boundary: ' + token)

for token in [
    'Packages',
    'Starter: USD 990/month + VAT',
    'Business: USD 1,490/month + VAT',
    'Premium: USD 2,490/month + VAT',
    'UNLIMITED',
    'annual_list_price',
    'annual_price',
    'TextDecoration.lineThrough',
]:
    require(token in module_ui or token in portal_ui, 'missing plan UI token: ' + token)

require('17,880' in acceptance and '16,390' in acceptance, 'Business annual full/discounted amounts are not documented')
require('29,880' in acceptance and '22,410' in acceptance, 'Premium annual full/discounted amounts are not documented')
require('UNLIMITED' in acceptance, 'Premium unlimited entitlement is not documented')
require('partner_plan_entitlement_policies' in central5_catalog, 'dynamic Premium entitlement policy storage is missing')
require('module recurring charge' in acceptance.lower(), 'acceptance does not state that modules are not recurring invoice authority')
require('attempt 1: due date / day 1' in acceptance.lower(), 'day-1/day-3/day-6 dunning schedule is not documented')
require('30-day cure window' in acceptance.lower(), 'dunning cure window is not documented')
require('financial invoices, payment settlements' in acceptance.lower(), 'legal-ledger retention carve-out is not documented')

for token in [
    '/api/v1/billing/plans:',
    '/api/v1/billing/plans/{planKey}:',
    '/api/v1/billing/partners/{partnerId}/plan:',
    '/api/v1/billing/partners/{partnerId}/plan/modules:',
]:
    require(token in openapi, 'OpenAPI missing plan route: ' + token)

print('HIMATE START-23.11.2 subscription-plan billing audit passed')
