#!/usr/bin/env python3
from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]
errors = []

billing = (ROOT / "services/cmd/billing/main.go").read_text()
commercial = (ROOT / "services/cmd/billing/commercial_automation.go").read_text()
catalog = (ROOT / "services/cmd/catalog/main.go").read_text()
portal = (ROOT / "services/cmd/gateway/partner_portal.go").read_text()
admin_ui = (ROOT / "frontend/lib/main.dart").read_text()
modules_ui = (ROOT / "frontend/lib/module_control_plane.dart").read_text()
localization = (ROOT / "frontend/lib/localization.dart").read_text()
render = (ROOT / "render.yaml").read_text()
matrix = json.loads((ROOT / "docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())

required_billing = [
    "start233BillingLifecycleMigration()",
    "lifecycle_state",
    "CANCEL_PENDING",
    "cancellation_requested_at",
    "cancellation_effective_at",
    "cancellation_requested_by",
    "cancellation_reason",
    "SUBSCRIPTION_INACTIVE",
    "MODULE_CANCELLATION_SCHEDULED",
    "MODULE_CANCELLATION_WITHDRAWN",
    "MODULE_CANCELLATION_EFFECTIVE",
]
for token in required_billing:
    if token not in billing and token not in commercial:
        errors.append(f"Billing missing START-23.3 lifecycle contract: {token}")

if 'BILLING_LIFECYCLE_REQUIRED' not in catalog:
    errors.append("Catalog does not block external ACTIVE -> NOT_LICENSED bypass")
if '*in.Status == "NOT_LICENSED" && old != "NOT_LICENSED" && !internal' not in catalog:
    errors.append("Catalog does not block all external transitions into NOT_LICENSED")

if 'fmt.Sprint(target["status"])!="ACTIVE"' in portal:
    errors.append("Partner Portal still gates cancellation on Catalog ACTIVE status")
if '"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/subscriptions/"' not in portal:
    errors.append("Partner Portal does not route cancellation to Billing")

if "Paid-period deactivation is Billing-managed" not in admin_ui:
    errors.append("Admin UI does not explain Billing-owned deactivation")
if "DropdownMenuItem(value: 'NOT_LICENSED'" not in admin_ui:
    errors.append("Admin UI lost inactive-state support instead of guarding only active deactivation")
if "if ('${module['status']}' == 'NOT_LICENSED')" not in admin_ui:
    errors.append("Admin UI can still offer NOT_LICENSED as a transition from a live or maintenance entitlement")

if "Subscription lifecycle" not in modules_ui or "cancellation_effective_at" not in modules_ui:
    errors.append("Modules commercial matrix does not expose Billing lifecycle state")

for token in [
    "'Subscription lifecycle': 'Előfizetés életciklusa'",
    "'Cancellation effective': 'Lemondás hatálybalépése'",
    "'Cancel pending': 'Lemondás függőben'",
]:
    if token not in localization:
        errors.append(f"START-23.3 Hungarian lifecycle localization missing: {token}")

required_render = [
    "name: himate-30day-invoice-cycle",
    'schedule: "0 6 * * *"',
    "dockerCommand: /app/service --run-invoice-cycle",
]
for token in required_render:
    if token not in render:
        errors.append(f"Render production billing scheduler missing: {token}")

contracts = {item["id"]: item for item in matrix.get("contracts", [])}
for key in ["PART-MODULE-EDIT","PART-MODULE-CANCEL","PORTAL-MODULE-CANCEL","BILLING-SCHEDULER","MODULE-CANCELLATION-CONSISTENCY"]:
    item = contracts.get(key)
    if not item:
        errors.append(f"Functional matrix missing {key}")
    elif item.get("target_phase") == "23.3" and item.get("current_state") not in {"MUTATION_PROVEN_PROD_UNVERIFIED","PROD_PROVEN"}:
        errors.append(f"{key} was not advanced after START-23.3 implementation")

if errors:
    raise SystemExit("START-23.3 audit failed:\n- " + "\n- ".join(errors))

print("START-23.3 audit passed: Billing-owned cancellation, Catalog bypass guard, shared portal/admin command path, lifecycle UI and production cron are present.")
