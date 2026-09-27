#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors = []

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        errors.append(f"missing file: {path}")
        return ""
    return p.read_text(encoding="utf-8")

def check(condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)

frontend = read("frontend/lib/main.dart")
localization = read("frontend/lib/localization.dart")
gateway = read("services/cmd/gateway/central10.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")

# Packages: authoritative data and reference interaction.
for token in [
    "for (final plan in canonicalPlans)",
    "class _PackageFeatureRow",
    "class _PackageComparisonTable",
    "Package comparison",
    "Most popular",
    "showPackageDetails(plan)",
    "editPackage(plan)",
    "analyticsByPlan",
    "_syncPackageMutation",
    "central10PlanDisplayPrice(",
    'central10Float(plan["monthly_price"])',
    'central10Int(plan["module_limit"])',
]:
    check(token in frontend or token in gateway, f"CENTRAL-17.2 Packages contract missing: {token}")

for forbidden in [
    "10 HIMATE-defined modules for focused teams",
    "20 HIMATE-defined modules for broader operating workflows",
    'out["monthly_price"] = 990',
    'out["monthly_price"] = 1490',
    'out["monthly_price"] = 2490',
]:
    check(forbidden not in frontend + "\n" + gateway, f"CENTRAL-17.2 stale package literal survived: {forbidden}")

# Finance: reference composition and functional selectors.
for token in [
    "class _FinanceInvoicePreview",
    "applyRevenuePeriod",
    "applyRevenuePlan",
    "applyInvoiceFilter",
    "scrollToInvoices",
    "invoiceKey",
    "financeChart()",
    "_syncFinanceMutation",
    "_invoiceMutationVisible",
    "_onboardingMutationVisible",
    "Recent invoice activity",
]:
    check(token in frontend, f"CENTRAL-17.2 Finance contract missing: {token}")

# Impact: explicit availability/permissions and bounded first-navigation recovery.
for token in [
    "impactSnapshotWarming",
    "_impactWarmRetryCount < 2",
    "_syncImpactMutation",
    "impactStatus == 'unavailable'",
    "impactStatus == 'partial'",
    "Some Impact sections are restricted",
    "canReadImpact",
    "canWriteImpact",
    "canReadEvidence",
    "canWriteEvidence",
    "canReadReports",
    "canWriteReports",
]:
    check(token in frontend, f"CENTRAL-17.2 Impact UI contract missing: {token}")

for token in [
    '"access": map[string]any{',
    'a.hasPermission(actor, "impact.read")',
    'a.hasPermission(actor, "impact.write")',
    'a.hasPermission(actor, "evidence.read")',
    'a.hasPermission(actor, "evidence.write")',
    'a.hasPermission(actor, "reports.read")',
    'a.hasPermission(actor, "reports.write")',
]:
    check(token in gateway, f"CENTRAL-17.2 Impact permission read-model contract missing: {token}")

check('successful == 0 && previous == nil' in step4 and 'status = "unavailable"' in step4,
      "CENTRAL-17.2 Impact materializer must persist an unavailable snapshot instead of warming forever")
check('status = "partial"' in step4,
      "CENTRAL-17.2 Impact materializer partial-state contract missing")

# Shared shell: Round 2 pages use the same reference hierarchy as Round 1.
for token in [
    "final referenceHeader = selected >= 0 && selected <=",
    "Subscription packages, module entitlements and configuration.",
    "Invoicing, receivables, licenses and partner onboarding overview.",
    "Real outcomes. Transparent reporting. Measurable impact.",
]:
    check(token in frontend, f"CENTRAL-17.2 shared shell contract missing: {token}")

# Critical bilingual strings introduced by Round 2.
for token in [
    "'Most popular':",
    "'Recent invoice activity':",
    "'Some Impact sections are restricted':",
    "'Subscription packages, module entitlements and configuration.':",
    "'Invoicing, receivables, licenses and partner onboarding overview.':",
    "'Real outcomes. Transparent reporting. Measurable impact.':",
    "String uiBilingual(String en, String hu)",
]:
    check(token in localization, f"CENTRAL-17.2 localization contract missing: {token}")

if errors:
    print(f"FAIL: CENTRAL-17.2 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-17.2 Packages/Finance/Impact reference + functional acceptance: PASS")
