#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def check(ok: bool, message: str) -> None:
    if not ok:
        raise SystemExit("CENTRAL-11 FAIL: " + message)

frontend = read("frontend/lib/main.dart")
modules = read("frontend/lib/module_control_plane.dart")
design = read("frontend/lib/design_guide.dart")
localization = read("frontend/lib/localization.dart")
gateway = read("services/cmd/gateway/main.go")
gateway_c10 = read("services/cmd/gateway/central10.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
partners = read("services/cmd/partners/main.go")
fixture = read("services/cmd/partners/test_fixture.go")
storage = read("services/cmd/storage/main.go")
billing6 = read("services/cmd/billing/central6.go")
billing8 = read("services/cmd/billing/central8.go")
billing11 = read("services/cmd/billing/central11.go")
automation_service = read("services/cmd/automation/main.go")
tenant_finance = read("services/cmd/tenantfinance/storage.go")
impact = read("services/cmd/impact/main.go")
render = read("render.yaml")
compose = read("docker-compose.yml")
openapi = read("docs/openapi.yaml")
acceptance = read("docs/CENTRAL-11_ACCEPTANCE.md")

# CENTRAL-21 first-click contract: no browser prewarm and no hidden page
# pre-mount. Presets are query-only views over the authoritative Partners
# snapshot, while the Gateway owns all data warming.
for token in [
    "String centralPartnersPresetPath(",
    "String centralFinancePath({",
    "void _warmControlPlane()",
    "Gateway owns authoritative read-model warming",
]:
    check(token in frontend, f"first-click canonical-path contract missing: {token}")
warm_start = frontend.find("void _warmControlPlane()")
warm_end = frontend.find("Future<void> _loadPublishedBrandAssets", warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ""
check("api.prefetch(" not in warm and "primaryTargets" not in warm and "deferredTargets" not in warm,
      "Central browser prewarm tiers survived")
check("_prebuildPriorityPages" not in frontend,
      "hidden workspace pre-mount still recreates a hard-refresh request wave")
check("centralStep3SnapshotGet(centralStep4PartnersKey)" in gateway_c10,
      "Partners presets are not served from the authoritative Partners snapshot")
check("refreshCentralStep4Partners" in step4,
      "Partners authoritative background materializer missing")

for class_name in [
    "class _PartnersPageState",
    "class _FinancePageState",
    "class _ImpactPageState",
]:
    start = frontend.find(class_name)
    check(start >= 0, f"{class_name} missing")
    snippet = frontend[start:start + 2200]
    check("bool loading = true;" in snippet, f"{class_name} does not start in truthful loading state")

# Packages: inactive selection must be explained by authoritative eligibility.
for token in [
    "s(m['publication_status']) == 'PUBLISHED'",
    "s(m['implementation_state']) == 'READY'",
    "Package modules are not ready yet",
    "Not enough PUBLISHED + READY modules",
    "candidates.length < limit",
]:
    check(token in modules, f"package eligibility UX contract missing: {token}")

# System & Operations caches its Future and exposes real diagnostics.
system_start = frontend.find("class SystemPage extends StatefulWidget")
system_state = frontend.find("class _SystemPageState", system_start)
system_end = frontend.find("\nclass ", system_state + 1)
system = frontend[system_start:system_end] if system_start >= 0 and system_state > system_start and system_end > system_state else ""
for token in [
    "late Future<Map<String, dynamic>> _future;",
    "_future = _load();",
    "centralSystemInitialPath()",
    "Developer diagnostics",
    "/api/v1/system-health/snapshot",
    "/api/v1/audit/events?outcome=FAILED&limit=20&offset=0",
    "correlation_id",
]:
    check(token in system, f"System diagnostics/cache contract missing: {token}")
check("Future.wait([" not in system[system.find("Future<Map<String, dynamic>> _load"):system.find("Future<void> _openDeveloperDiagnostics")],
      "System initial overview regressed to a Flutter-side API waterfall")

# Design Guide / CMS real preview and fixed responsive viewport frame.
for token in [
    "createPreview('desktop')",
    "Click the preview to open the real draft website in a new tab.",
    "!widget.canWrite",
]:
    check(token in design, f"Design Guide actionable permission-safe preview missing: {token}")
for token in [
    'width := "1440px"',
    'height := "900px"',
    'width = "834px"',
    'height = "1194px"',
    'width = "390px"',
    'height = "844px"',
]:
    check(token in gateway, f"real preview viewport contract missing: {token}")

for token in [
    "'Content and system logic remain unchanged when the layout family changes.'",
    "'Tablet preview': 'Tablet előnézet'",
    "'Mobile preview': 'Mobil előnézet'",
    "'Sequence Apply Case': 'Szekvencia alkalmazási eset'",
    "if (value.contains(' · '))",
]:
    check(token in localization, f"Hungarian CMS/archive localization missing: {token}")

# Golden Test Partner fixture and hard-reset safety.
for token in [
    "testFixtureMarker = \"HIMATE_GOLDEN_TEST_FIXTURE\"",
    "seedTestPartnerFixture",
    "purgeTestPartner",
    "set_config('himate.test_partner_purge',$1,TRUE)",
    "if !p.TestPartner",
    "confirm_partner_id",
    "COMPLIANCE_RETENTION",
    "TEST_REPORT_IN_PROGRESS",
    "purgeTestReportObjects",
    "purgeTestStorage",
    "audit_receipt_preserved",
    "qa.piano.pianos_serviced",
    "qa.piano.technician_hours",
    "qa.piano.customer_jobs",
    "PARTNER_IMPACT",
]:
    check(token in fixture, f"Golden Test Partner fixture/reset contract missing: {token}")

for suffix in ["seed-test-fixture", "purge-test-fixture"]:
    check(suffix in partners, f"Partners route missing: {suffix}")
    check(suffix in gateway, f"System Owner gateway protection missing: {suffix}")
    check(f"/api/v1/partners/{{partnerId}}/{suffix}:" in openapi,
          f"OpenAPI path missing: {suffix}")

check("!u.SystemOwner" in gateway[gateway.find("seed-test-fixture")-500:gateway.find("purge-test-fixture")+500],
      "Golden Test Partner maintenance is not System Owner protected")

for token in [
    'action == "purge-test"',
    "X-Himate-Test-Partner-Confirm",
    "test_partner",
    "os.RemoveAll(root)",
]:
    check(token in storage, f"storage Test Partner purge contract missing: {token}")

# Partners must reach storage in both production and Compose topology.
partners_render = render[render.find("name: himate-partners"):render.find("\n  - type:", render.find("name: himate-partners") + 1)]
check("key: STORAGE_HOSTPORT" in partners_render and "name: himate-storage" in partners_render,
      "Render Partners->Storage binding missing")
compose_lines = compose.splitlines()
partners_start = next((i for i, line in enumerate(compose_lines) if line == "  partners:"), -1)
partners_end = next(
    (i for i in range(partners_start + 1, len(compose_lines))
     if compose_lines[i].startswith("  ") and not compose_lines[i].startswith("    ") and compose_lines[i].endswith(":")),
    len(compose_lines),
)
partners_compose = "\n".join(compose_lines[partners_start:partners_end]) if partners_start >= 0 else ""
check("STORAGE_HOSTPORT: storage:10000" in partners_compose,
      "Compose Partners->Storage binding missing")

# Golden Test data must not pollute HIMATE aggregates.
check("COALESCE(p.test_partner,FALSE)=FALSE" in billing6,
      "Finance KPI/onboarding aggregate does not exclude Test Partner")
check("tp.test_partner=TRUE" in billing6,
      "Finance monthly chart does not exclude Test Partner")
check("tp.test_partner=TRUE" in billing8,
      "Finance revenue trends do not exclude Test Partner")
check("COALESCE(p.test_partner,FALSE)=FALSE" in impact,
      "Impact global aggregate does not exclude Test Partner")

# Immutable production ledgers may be deleted only inside the transaction-scoped
# Golden Test Partner purge capability.
for token in [
    "central11BillingTestPurgeMigration",
    "Version: 20",
    "current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id",
]:
    check(token in billing11, f"Billing Test Partner purge guard missing: {token}")
check('Version:2,Name:"central-11-golden-test-partner-purge-guard"' in automation_service,
      "Automation Test Partner purge migration missing")
check("current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id" in automation_service,
      "Automation immutable event purge is not target-scoped")
check('Version: 3, Name: "central-11-golden-test-partner-purge-guard"' in tenant_finance,
      "Tenant Finance Test Partner purge migration missing")
check("current_setting('himate.test_partner_purge', TRUE)=OLD.partner_id" in tenant_finance,
      "Tenant Finance append-only purge is not target-scoped")

# Existing Administration audit stream and separate Compliance Archives remain intact.
check("Administrative Event Stream" in frontend, "Administration audit event stream disappeared")
check("seven-year Compliance Archive" in acceptance, "Central-11 does not preserve archive separation")
check("future corporate document center" in acceptance.lower(),
      "Central-11 acceptance does not explicitly keep corporate documents separate")

check("CENTRAL-11" in acceptance and "smoke_central_11.sh" in acceptance,
      "CENTRAL-11 acceptance document is incomplete")

print("CENTRAL-11 static acceptance passed")
