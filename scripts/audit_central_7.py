#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
errors: list[str] = []

def read(path: str) -> str:
    return (ROOT / path).read_text()

def check(ok: bool, message: str) -> None:
    if not ok:
        errors.append(message)

ci = read(".github/workflows/ci.yml")
central1 = read("scripts/audit_central_1.py")
central2 = read("scripts/audit_central_2.py")
central3 = read("scripts/audit_central_3.py")
central4 = read("scripts/audit_central_4.py")
central5 = read("scripts/audit_central_5.py")
central6 = read("scripts/audit_central_6.py")

catalog = read("services/cmd/catalog/main.go")
catalog_marketplace = read("services/cmd/catalog/marketplace.go")
catalog_plans = read("services/cmd/catalog/plans.go")
billing_plans = read("services/cmd/billing/plans.go")
dunning = read("services/cmd/billing/dunning.go")
central6_backend = read("services/cmd/billing/central6.go")
partner_gateway = read("services/cmd/gateway/partner_portal.go")
gateway_security = read("services/cmd/gateway/security_phase4.go")
partners = read("services/cmd/partners/main.go")
impact = read("services/cmd/impact/main.go")

for number in range(1, 7):
    path = ROOT / f"scripts/audit_central_{number}.py"
    check(path.exists(), f"CENTRAL-{number} static acceptance is missing")

for number in range(2, 7):
    path = ROOT / f"scripts/smoke_central_{number}.sh"
    check(path.exists(), f"CENTRAL-{number} runtime acceptance is missing")

static_tokens = [f"python3 scripts/audit_central_{n}.py" for n in range(1, 8)]
static_pos = [ci.find(token) for token in static_tokens]
for token, pos in zip(static_tokens, static_pos):
    check(pos >= 0, f"CI static gate missing: {token}")
if all(pos >= 0 for pos in static_pos):
    check(static_pos == sorted(static_pos), "CENTRAL-1..7 static gates are not ordered deterministically")

runtime_tokens = [f"sh scripts/smoke_central_{n}.sh http://127.0.0.1:8080" for n in range(2, 8)]
runtime_pos = [ci.find(token) for token in runtime_tokens]
for token, pos in zip(runtime_tokens, runtime_pos):
    check(pos >= 0, f"CI runtime gate missing: {token}")
if all(pos >= 0 for pos in runtime_pos):
    check(runtime_pos == sorted(runtime_pos), "CENTRAL-2..7 runtime gates are not ordered deterministically")

build_pos = ci.find("Build and start containerized microservices")
if static_pos[-1] >= 0 and build_pos >= 0:
    check(static_pos[-1] < build_pos, "CENTRAL-7 static audit must run before the Compose runtime build")

mfa_match = re.search(r"func partnerMFARequired\(_ string\) bool \{(.*?)\n\}", gateway_security, re.S)
check(mfa_match is not None and "return false" in mfa_match.group(1),
      "Partner Portal MFA is no longer optional before module activation")
check('"two_factor_authentication"' in catalog,
      "Planned two_factor_authentication module disappeared from Module Registry")

check(impact.count("COALESCE(p.test_partner,FALSE)=FALSE") >= 3,
      "Dashboard/Impact no longer excludes Test Partners from production aggregation")
partners_start = partners.find("func (a *app) partners")
partners_scope = partners[partners_start:] if partners_start >= 0 else partners
check("p.test_partner=FALSE" not in partners_scope and "p.test_partner = FALSE" not in partners_scope,
      "Partners read model hides Test Partners")

check("group_key" in catalog and "seedGroups" in catalog,
      "Module Registry category/group authority is missing")
for token in ['"fixed_module_keys"', "selectionModeUnlimited", '"entitlement_mode"']:
    check(token in billing_plans + catalog_plans,
          f"Package/module entitlement contract missing: {token}")
check("availablePublishedModuleKeys" in billing_plans,
      "Premium Unlimited no longer derives from the released module catalog")
check("fixed_module_keys" in partner_gateway,
      "Partner Portal package/module projection is disconnected from fixed package sets")

for forbidden in [
    "expected 40",
    "len(seedModules) != 40",
    "len(marketplaceSummaries) != 40",
    "len(canonical)==40",
    "len(system)==40",
    "len(legacy)==38",
    'test "$ACTIVE_COUNT" = "38"',
]:
    check(forbidden not in catalog + catalog_marketplace + billing_plans + catalog_plans,
          f"Fixed module cardinality regressed into production/domain source: {forbidden}")

for token in ["workflow_status,source", "'DRAFT','AUTOMATED'"]:
    check(token in billing_plans, f"Recurring invoices no longer start as approval drafts: {token}")
check("workflow_status IN ('SENT','PAID')" in dunning,
      "Dunning can operate outside SENT/PAID invoice states")
check("Only SENT invoices can be marked paid" in central6_backend,
      "CENTRAL-6 allows payment before invoice distribution")

check('"/internal/v1/partners/"+url.PathEscape(partnerID)+"/portal-gate"' in partner_gateway,
      "Partner Portal login no longer calls the billing onboarding gate")
check("state.State == onboardingActive && state.PortalEnabled" in central6_backend,
      "CENTRAL-6 Portal gate is no longer ACTIVE + portal_enabled")

detail_match = re.search(r"func \(a \*app\) invoiceDetail\(.*?\n\}", central6_backend, re.S)
check(detail_match is not None, "CENTRAL-6 invoiceDetail serializer is missing")
if detail_match is not None:
    detail = detail_match.group(0)
    for field in [
        '"payment_attempt_id"', '"provider"', '"provider_payment_id"',
        '"payment_failure_code"', '"payment_failure_message"',
        '"collection_attempts"', '"dunning_state"',
        '"dunning_suspended_at"', '"purge_due_at"', '"operational_purged_at"',
    ]:
        check(field in detail, f"Invoice detail drifted from canonical payment/dunning contract: {field}")
    check("provider='MANUAL'" in central6_backend and '"provider":provider' in detail,
          "Manual payment provider does not round-trip through invoice detail")

for token, body, label in [
    ("Partner Portal", central1, "CENTRAL-1 public entry"),
    ("weekly_trend", central2, "CENTRAL-2 weekly Impact"),
    ("Test Partners", central3, "CENTRAL-3 test partner visibility"),
    ("module_usage_events", central4, "CENTRAL-4 runtime usage"),
    ("Unlimited", central5, "CENTRAL-5 Premium Unlimited"),
    ("payment_deadline_at", central6, "CENTRAL-6 payment deadline"),
]:
    check(token in body, f"{label} acceptance evidence disappeared")

if errors:
    print(f"FAIL: CENTRAL-7 first-half regression closure found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-7 Central-1..6 static regression closure: PASS")
