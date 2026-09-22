#!/usr/bin/env python3
from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]
errors = []

catalog = (ROOT / "services/cmd/catalog/main.go").read_text()
billing = (ROOT / "services/cmd/billing/main.go").read_text()
gateway = (ROOT / "services/cmd/gateway/main.go").read_text()
ui = (ROOT / "frontend/lib/module_control_plane.dart").read_text()
main = (ROOT / "frontend/lib/main.dart").read_text()
matrix = json.loads((ROOT / "docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())

required_catalog = [
    "default_activation_fee",
    "activation_fee_override",
    "catalog.activation_fee_history",
    "/api/v1/module-commercial-matrix",
    "partnerModuleCommercialHistory",
    "/internal/v1/module-price-quotes",
    "resolvePartnerModulePriceAt",
]
for token in required_catalog:
    if token not in catalog:
        errors.append(f"Catalog missing START-23.2 contract: {token}")

required_billing = [
    "/api/v1/billing/subscription-matrix",
    "catalogPriceQuotes",
    "next_billing_date",
    "next_period_price",
    "next_period_included_in_base",
    "current_period_included_in_base",
]
for token in required_billing:
    if token not in billing:
        errors.append(f"Billing missing START-23.2 contract: {token}")

for token in [
    "/api/v1/module-commercial-matrix",
    "/api/v1/billing/",
]:
    if token not in gateway:
        errors.append(f"Gateway missing START-23.2 route boundary: {token}")

required_ui = [
    "Partner × Module Commercial Matrix",
    "View by partner",
    "View by module",
    "Next billing date",
    "Next billing price",
    "Commercial history",
    "Partner activation fee",
    "Default activation fee",
    "/api/v1/module-commercial-matrix",
    "/api/v1/billing/subscription-matrix",
]
for token in required_ui:
    if token not in ui:
        errors.append(f"Module Control Plane missing START-23.2 UI contract: {token}")

legacy_tokens = [
    "await widget.api.post('/api/v1/modules'",
    "await widget.api.patch('/api/v1/modules/${module['key']}'",
]
for token in legacy_tokens:
    if token in main:
        errors.append(f"Legacy Finance module mutation path still exists: {token}")

contracts = {item["id"]: item for item in matrix.get("contracts", [])}
if "MODULE-COMMERCIAL-MATRIX-EDIT" not in contracts:
    errors.append("Functional matrix missing MODULE-COMMERCIAL-MATRIX-EDIT")
for old in ["FINANCE-LEGACY-MODULE-CREATE", "FINANCE-LEGACY-MODULE-EDIT"]:
    if old in contracts:
        errors.append(f"Resolved duplicate contract still present: {old}")

if contracts.get("PART-MODULE-EDIT", {}).get("target_phase") != "23.3":
    errors.append("PART-MODULE-EDIT entitlement semantics must remain assigned to START-23.3")

if errors:
    raise SystemExit("START-23.2 audit failed:\n- " + "\n- ".join(errors))

print("START-23.2 audit passed: commercial matrix, individual pricing, activation fees, exact next-period quote, history and duplicate-path removal are present.")
