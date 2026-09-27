#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
failures = []

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")

def check(ok: bool, message: str) -> None:
    if not ok:
        failures.append(message)

workflow = read(".github/workflows/ci.yml")
main = read("services/cmd/gateway/main.go")
reads = read("services/cmd/gateway/central_materialized_reads.go")
notifications = read("services/cmd/gateway/central_notification_read_models.go")
models = read("services/cmd/gateway/materialized_read_models.go")
readiness = read("services/cmd/gateway/read_model_readiness.go")
seeds = read("services/cmd/gateway/read_model_seeds.go")
central21 = read("scripts/smoke_central_21.sh")
start0913 = read("scripts/smoke_start_09_13.sh")

# Inventory every shell smoke/integration script the Compose job can execute.
scripts = []
for match in re.finditer(r"\b(?:sh|bash)\s+(scripts/[A-Za-z0-9_.-]+\.sh)\b", workflow):
    path = match.group(1)
    if path not in scripts:
        scripts.append(path)

check(len(scripts) >= 55, f"CI smoke inventory unexpectedly small: {len(scripts)}")
for path in scripts:
    check((ROOT / path).is_file(), f"CI references missing smoke/integration script: {path}")

# Historical compatibility probes that must remain authoritative and immediate.
for token in [
    "/api/v1/environments?partner_id=",
    "/api/v1/system-health",
    "/api/v1/provisioning/jobs",
]:
    check(token in start0913, f"START-09-13 compatibility probe disappeared: {token}")
check("X-Himate-Locale" not in start0913,
      "START-09-13 was accidentally converted from legacy compatibility mode to browser CQRS mode")

# One URL contract, two deterministic read modes:
# browser API client -> persistent CQRS, curl/smoke -> authoritative owner API.
check("func centralBrowserMaterializedRead" in reads,
      "Browser CQRS discriminator helper is missing")
check('r.Header.Get("X-Himate-Locale")' in reads,
      "Browser CQRS discriminator is no longer based on the shipped client header")
check("if !centralBrowserMaterializedRead(r)" in reads,
      "Generic materialized GET interceptor can still capture legacy/smoke reads")
check('case r.URL.Path == "/api/v1/environments", strings.HasPrefix(r.URL.Path, "/api/v1/environments/"):' in main
      and 'a.serveProxy(w, r, "environments")' in main,
      "Legacy environment GET no longer reaches authoritative Environment service")
check('case strings.HasPrefix(r.URL.Path, "/api/v1/provisioning/"):' in main
      and 'a.serveProxy(w, r, "provisioning")' in main,
      "Legacy provisioning GET no longer reaches authoritative Provisioning service")
check('case strings.HasPrefix(r.URL.Path, "/api/v1/system-health"):' in main
      and 'a.serveProxy(w, r, "health")' in main,
      "Legacy system-health GET no longer reaches authoritative Health snapshot service")
check('case r.URL.Path == "/api/v1/partners", r.URL.Path == "/api/v1/partner-categories":' in main,
      "Legacy partner/category compatibility route is missing")
check('case r.URL.Path == "/api/v1/partners" && r.Method == http.MethodGet:' not in main,
      "Legacy partner GET is still diverted into the Central materialized portfolio handler")
check("!centralBrowserMaterializedRead(r)" in notifications,
      "Notifications can still capture legacy/smoke GETs with a stale user projection")

# CENTRAL-21 runtime explicitly emulates the shipped browser client when it
# validates zero-fanout generic REST reads.
check("-H 'X-Himate-Locale: en'" in central21,
      "CENTRAL-21 browser CQRS smoke does not send the browser discriminator")

# Cold-start deterministic readiness must cover all Central and tenant models.
for token in [
    "centralStep3RegistryKey", "centralStep3PlansKey", "centralStep3AnalyticsKey",
    "centralStep3CommercialKey", "centralStep4PartnersKey", "centralStep4FinanceKey",
    "centralStep4ImpactKey", "centralStep4AdministrationKey", "centralStep4SystemKey",
    "centralStep4WebsiteKey", "centralStep4ConnectionsKey", "centralStep4ComplianceKey",
    "centralStep4GlobalSearchKey",
]:
    check(token in seeds and token in readiness,
          f"Central cold-start seed/readiness coverage missing: {token}")
check("seedPartnerWorkspaceBaseline" in seeds and "ensurePartnerReadModelsReady" in readiness,
      "Tenant cold-start seed/readiness contract is incomplete")

# Immediate mutation -> browser F5 consistency and legacy health compatibility.
check("a.refreshHealthSourceWriteThrough()" in models,
      "System/partner writes no longer synchronously refresh persistent Health compatibility state")
check('strings.Contains(reason, "environment") || strings.Contains(reason, "provision")' in models,
      "Provisioning write-through no longer refreshes environment-bearing browser projections")
check('refreshReason += "/audit"' in main,
      "Mutation write-through no longer includes the newly committed audit event")
check(main.find("finalizeAuditIntent") < main.find("writeThroughReadModels"),
      "Audit durability finalization occurs after materialized write-through")

if failures:
    print(f"SMOKE/CQRS COMPAT FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print(f"SMOKE/CQRS compatibility audit: PASS ({len(scripts)} CI shell scripts inventoried)")
