#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
failures = []

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")

def check(ok: bool, message: str) -> None:
    if not ok:
        failures.append(message)

billing18 = read("services/cmd/billing/central18.go")
billing_main = read("services/cmd/billing/main.go")
billing8 = read("services/cmd/billing/central8.go")
gateway10 = read("services/cmd/gateway/central10.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
dashboard = read("services/cmd/gateway/dashboard_snapshot.go")
health = read("services/cmd/health/main.go")
system = read("services/cmd/gateway/central17_round3.go")
admin = read("services/cmd/gateway/central14.go")
frontend = read("frontend/lib/main.dart")
modules = read("frontend/lib/module_control_plane.dart")
usmap = read("frontend/lib/central17_round1.dart")
backups = read("frontend/lib/backups_panel.dart")
localization = read("frontend/lib/localization.dart")
partner_fixture = read("services/cmd/partners/test_fixture.go")
partners_main = read("services/cmd/partners/main.go")

for token in [
    "Version: 21",
    "'STARTER',500::numeric,990::numeric",
    "'BUSINESS',1500::numeric,1490::numeric",
    "'FLEX',2500::numeric,2490::numeric",
    "module_limit=10",
    "module_limit=20",
    "selection_mode='UNLIMITED'",
    "change_type='CENTRAL18_RECOVERY'",
    "COALESCE(l.change_type,'')<>'MANUAL'",
]:
    check(token in billing18, f"package recovery contract missing: {token}")
check("central18BillingPackageRecoveryMigration()" in billing_main, "CENTRAL-18 billing migration is not registered")
check("WHERE s.plan_key IN ('STARTER','BUSINESS','FLEX')" not in billing8, "custom active package subscriptions are still hidden from analytics")

for token in [
    "partners, partnerErr = a.central10AllPartners(ctx)",
    '"items":          partners',
    "lifecycleCounts",
]:
    check(token in step4, f"complete Partners snapshot contract missing: {token}")
for token in [
    "every Partners view is served from the complete enriched",
    "centralStep3SnapshotGet(centralStep4PartnersKey)",
    "searchContains(",
    '"pagination": map[string]any{',
]:
    check(token in gateway10, f"snapshot-filtered Partners contract missing: {token}")

for token in [
    "dashboardAllUSStates",
    '"Alaska"',
    '"Hawaii"',
    '"active_states":   activeStates',
]:
    check(token in dashboard, f"US geography contract missing: {token}")
for token in [
    "_nearestState(",
    "onTapUp:(details)",
    "No partner records in this state",
    "'Alaska':'AK'",
]:
    check(token in usmap, f"interactive US map contract missing: {token}")

for token in [
    "initialPartnerId",
    "PartnerRouteLoader(",
    "_openWorkspaceSection(",
    "initialData: api.peek(path)",
    "evidence = items(<String, dynamic>{'items': model['evidence']})",
]:
    check(token in frontend, f"partner refresh/workspace contract missing: {token}")
check("_WorkspaceSpec('Evidence', Icons.verified_outlined, 'Impact evidence library', true)" in frontend,
      "Partner Evidence workspace is not active")
check('runPage("evidence", "evidence"' in gateway10, "partner Evidence is not backed by the Evidence service")

for token in [
    "Future<void> activateModule(",
    "'availability': 'ACTIVE'",
    "'implementation_state': 'READY'",
    "'publication_status': 'PUBLISHED'",
    "modules-add-module-button",
    "module-detail-activate-button",
    "Marketplace active",
]:
    check(token in modules, f"module lifecycle control missing: {token}")
check("'Module registry': 'Modulnyilvántartás'" in localization, "Module registry Hungarian terminology regressed")

for token in [
    '"backups_write":',
    'a.hasPermission(actor, "backups.write")',
]:
    check(token in system, f"backup write access missing from System read model: {token}")
for token in [
    "this.canApproveRestore",
    "widget.canApproveRestore ?? widget.canMutate",
    "Create restore point",
]:
    check(token in backups, f"backup creation/restore permission split missing: {token}")
check("if (canBackups) '_platform'" in frontend, "System backup panel does not expose the platform restore-point scope")

check("centralStep3SnapshotGet(centralStep4PartnersKey)" in admin,
      "Administration refresh does not reuse the materialized partner snapshot")
check(".timeout(const Duration(seconds: 6))" in read("frontend/lib/administration_center.dart"),
      "Administration frontend can still wait without a bounded timeout")

for token in [
    "if len(services)<=1",
    "services=a.checkServices(ctx)",
    "a.partnerHealth(ctx)",
]:
    check(token in health, f"cold health snapshot self-heal missing: {token}")

check("_applyPackageMutationImmediately(updated)" in frontend,
      "package mutation is not applied immediately on the frontend")
check("_packageMutationMatches(refreshed, updated)" in frontend,
      "package mutation reconciliation contract missing")
check("isExpanded: true" in frontend, "partner filter dropdown overflow guard missing")

for token in [
    "func (a *app) runGoldenTestFixtureReconciler()",
    "func (a *app) reconcileGoldenTestFixtures(",
    "HIMATE_GOLDEN_TEST_FIXTURE",
    "WHERE test_partner=TRUE",
    "fixtures := []monthFixture{",
]:
    check(token in partner_fixture, f"automatic Golden Test Partner fixture contract missing: {token}")
check("go a.runGoldenTestFixtureReconciler()" in partners_main,
      "Golden Test Partner fixture reconciler is not started by the Partners service")

if failures:
    print(f"CENTRAL-18 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-18 data/runtime/module lifecycle static acceptance: PASS")
