#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def check(ok: bool, message: str) -> None:
    if not ok:
        raise SystemExit("CENTRAL-12 FAIL: " + message)

frontend = read("frontend/lib/main.dart")
gateway = read("services/cmd/gateway/central10.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
billing8 = read("services/cmd/billing/central8.go")
partners8 = read("services/cmd/partners/central8.go")
impact8 = read("services/cmd/impact/central8.go")
acceptance = read("docs/CENTRAL-12_ACCEPTANCE.md")

for token in [
    'centralStep4PartnersKey       = "partners_screen"',
    "{centralStep4PartnersKey, a.refreshCentralStep4Partners}",
    "func (a *app) refreshCentralStep4Partners()",
    "partners, partnerErr = a.central10AllPartners(ctx)",
    '"items":          partners',
    'a.centralStep3Store(persistCtx, centralStep4PartnersKey, payload)',
]:
    check(token in step4, f"Partners materialization contract missing: {token}")

for token in [
    "centralStep3SnapshotGet(centralStep4PartnersKey)",
    "snapshotItems := step4Items(snapshot[\"items\"])",
    "searchContains(",
    '"X-Himate-Cache", "hot-snapshot"',
    'delete(row, "base_service_fee")',
    'delete(row, "active_modules")',
    'delete(row, "system_health")',
    '"pagination": map[string]any{',
]:
    check(token in gateway, f"Partners hot-read/filter/RBAC contract missing: {token}")

check('case strings.Contains(path, "module"), strings.Contains(path, "catalog"):' in gateway,
      "catalog invalidation path is missing")
catalog_case = gateway[gateway.find('case strings.Contains(path, "module"), strings.Contains(path, "catalog"):'):]
catalog_case = catalog_case[:catalog_case.find("case ", 10)] if "case " in catalog_case[10:] else catalog_case
check("a.requestCentralStep4Refresh()" in catalog_case,
      "catalog changes do not refresh the Partners materialization")

finance_start = frontend.find("class _FinancePageState")
finance_end = frontend.find("\nclass ", finance_start + 1)
finance = frontend[finance_start:finance_end]
for token in [
    "int _warmRetryCount = 0;",
    "Timer? _warmRetry;",
    "if (_warmRetryCount < 2)",
    "Duration(milliseconds: 900 * _warmRetryCount)",
    "Finance snapshot is warming",
]:
    check(token in finance, f"bounded Finance warming contract missing: {token}")
check("Duration(milliseconds: 400)" not in finance,
      "legacy unbounded 400 ms Finance polling remains")

for token in [
    "final primaryTargets = <String>{};",
    "final deferredTargets = <String>{};",
    "Duration(milliseconds: 1500)",
    "api.prefetch(primaryTargets",
    "api.prefetch(deferredTargets",
]:
    check(token in frontend, f"staged prewarm contract missing: {token}")

for source, name in [
    (partners8, "Partners"),
    (billing8, "Billing"),
    (impact8, "Impact"),
]:
    check('r.URL.Query().Get("availability") == "1"' in source,
          f"{name} PDF availability probe missing")
    check('"has_data": len(tableRows) > 0' in source,
          f"{name} PDF availability response missing")

for token in [
    "Future<void> openPdfExportIfAvailable(",
    "'availability': '1'",
    "No exportable data",
    "openPdfExportIfAvailable(context, widget.api, _partnerExportUri().toString())",
    "'/api/v1/billing/packages/export.pdf'",
    "openPdfExportIfAvailable(context, widget.api, financeExportPath)",
    "openPdfExportIfAvailable(context, widget.api, '/api/v1/impact/export.pdf')",
]:
    check(token in frontend, f"PDF empty-state UI contract missing: {token}")

step3 = read("services/cmd/gateway/central_step3_snapshots.go")
check('centralStep3AnalyticsKey     = "package_analytics"' in step3,
      "Package Analytics materialized snapshot key missing")
check('"/api/v1/billing/packages/analytics"' in step3,
      "Package Analytics is not sourced from Billing runtime data")
check("CENTRAL-12" in acceptance and "smoke_central_12.sh" in acceptance,
      "CENTRAL-12 acceptance document incomplete")

print("CENTRAL-12 static acceptance passed")
