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
workspace = read("services/cmd/gateway/central_partner_workspace_snapshots.go")
billing = read("services/cmd/billing/main.go")
frontend = read("frontend/lib/main.dart")
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
script_sources = {path: read(path) for path in scripts}

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
# explicit browser discriminator -> persistent CQRS; historical curl/smoke -> authoritative owner API.
# Locale alone is intentionally NOT a discriminator because START-23.5/23.7 are localized legacy smokes.
check("func centralBrowserMaterializedRead" in reads,
      "Browser CQRS discriminator helper is missing")
check('r.Header.Get("X-Himate-Read-Model")' in reads and '"browser"' in reads,
      "Browser CQRS discriminator is not explicit")
check('r.Header.Get("X-Himate-Locale")' in reads,
      "Browser CQRS locale requirement disappeared")
check("'X-Himate-Read-Model': 'browser'" in frontend,
      "Shipped Flutter browser client does not send the explicit CQRS discriminator")
for path, source in script_sources.items():
    if path != "scripts/smoke_central_21.sh":
        check("X-Himate-Read-Model" not in source,
              f"Historical smoke was accidentally converted to browser CQRS mode: {path}")
check("X-Himate-Locale" in script_sources.get("scripts/smoke_start_23_5.sh", ""),
      "START-23.5 localized legacy compatibility coverage disappeared")
check("X-Himate-Locale" in script_sources.get("scripts/smoke_start_23_7.sh", ""),
      "START-23.7 localized legacy compatibility coverage disappeared")
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
check("-H 'X-Himate-Locale: en'" in central21 and "-H 'X-Himate-Read-Model: browser'" in central21,
      "CENTRAL-21 browser CQRS smoke does not send both browser discriminators")

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
check("seedCentralUserNotificationReadModelBaselines" in notifications
      and "readModelSeeded" in notifications
      and "seedCentralUserNotificationReadModelBaselines(ctx)" in main,
      "Central user notification cold-start seed/readiness contract is incomplete")

# Projection builders must be read-only with respect to subscription lifecycle.
check('base+"/summary?read_model_source=1"' in workspace,
      "Partner workspace still calls the mutating legacy Billing summary path")
summary_start = billing.find("func (a *app) summary")
summary_end = billing.find("\nfunc ", summary_start + 1)
summary_block = billing[summary_start:summary_end if summary_end > summary_start else len(billing)]
check("read_model_source" in summary_block
      and "if !readModelSource" in summary_block
      and "syncSubscriptions" in summary_block,
      "Billing summary does not isolate read-model materialization from lifecycle synchronization")

# Immediate mutation -> browser F5 consistency and legacy health compatibility.
check("a.refreshHealthSourceWriteThrough()" in models,
      "System/partner writes no longer synchronously refresh persistent Health compatibility state")
check('strings.Contains(reason, "environment") || strings.Contains(reason, "provision")' in models,
      "Provisioning write-through no longer refreshes environment-bearing browser projections")
check('refreshReason += "/audit"' in main,
      "Mutation write-through no longer includes the newly committed audit event")
api_start = main.find("func (a *app) api")
api_end = main.find("\nfunc ", api_start + 1)
api_block = main[api_start:api_end if api_end > api_start else len(main)]
check(api_start >= 0 and
      api_block.find("finalizeAuditIntent") >= 0 and
      api_block.find("writeThroughReadModels") >= 0 and
      api_block.find("finalizeAuditIntent") < api_block.find("writeThroughReadModels"),
      "Audit durability finalization occurs after materialized write-through")

if failures:
    print(f"SMOKE/CQRS COMPAT FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print(f"SMOKE/CQRS compatibility audit: PASS ({len(scripts)} CI shell scripts inventoried)")
