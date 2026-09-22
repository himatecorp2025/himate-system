#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
billing = (root / "services/cmd/billing/main.go").read_text()
automation = (root / "services/cmd/billing/commercial_automation.go").read_text()
frontend = (root / "frontend/lib/main.dart").read_text()
openapi = (root / "docs/openapi.yaml").read_text()

def require(condition, message):
    if not condition:
        raise SystemExit("START-23.11.2 audit failed: " + message)

for token in [
    "start23112CalendarMonthBillingMigration()",
    "billing_cycle_model TEXT NOT NULL DEFAULT 'CALENDAR_MONTH'",
    "billing_model TEXT NOT NULL DEFAULT 'LEGACY_30_DAY'",
    "minimum_commitment_adjustment NUMERIC(12,2) NOT NULL DEFAULT 0",
    "pricing_effective_at DATE",
]:
    require(token in automation or token in billing, f"missing calendar-month migration token: {token}")

for token in [
    "func calendarMonthWindow",
    "func previousCalendarMonth",
    "return at.Day() == 1",
    '"billing_cycle_model":"CALENDAR_MONTH"',
    '"proration":"NONE"',
    '"invoice_timing":"NEXT_MONTH_DAY_1_FOR_PREVIOUS_CALENDAR_MONTH"',
]:
    require(token in billing, f"missing billing-cycle contract token: {token}")

require("nextStart.AddDate(0, 1, 0)" in billing, "module renewal must advance by calendar month")
sync_block = billing.split("func (a *app) syncSubscriptions",1)[1].split("func nullableTimeValue",1)[0]
require("AddDate(0, 0, 30)" not in sync_block, "subscription synchronization still contains fixed 30-day renewal logic")
require("minimumCommitmentAdjustment(base+moduleTotal, minimum)" in automation, "invoice assembly must enforce the negotiated minimum monthly commitment")
require("'MINIMUM_COMMITMENT'" in automation, "minimum monthly commitment adjustment invoice item is missing")
require("'CALENDAR_MONTH'" in automation, "calendar-month ledger marker is missing")
require('"proration": "NONE"' in automation or '"proration":"NONE"' in automation, "module-period evidence must explicitly record no proration")
require("pricingAt := activation" in billing, "mid-month activation must resolve the contractual price at activation")
require("cancellation_effective_at=CASE WHEN cancel_at_period_end THEN $4 ELSE NULL END" in billing, "pending cancellations must migrate to the calendar-month boundary")

for bad in [
    "activation-date anchored 30-day cycle",
    "Automatic 30-day collection",
    "Paid-period deactivation is Billing-managed. Use Cancel at period end; access remains active until the current 30-day period closes.",
]:
    require(bad not in frontend, f"obsolete billing UI language remains: {bad}")

require("calendar month" in openapi.lower(), "OpenAPI must document calendar-month billing")
require("no proration" in openapi.lower(), "OpenAPI must document no-proration billing")

print("HIMATE START-23.11.2 calendar-month billing audit passed")
