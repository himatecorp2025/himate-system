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

def func_block(source: str, signature: str) -> str:
    start = source.find(signature)
    if start < 0:
        return ""
    brace = source.find("{", start)
    if brace < 0:
        return ""
    depth = 0
    for i in range(brace, len(source)):
        if source[i] == "{":
            depth += 1
        elif source[i] == "}":
            depth -= 1
            if depth == 0:
                return source[start:i + 1]
    return source[start:]

main = read("services/cmd/gateway/main.go")
snapshots = read("services/cmd/gateway/central_step3_snapshots.go")
tenant_snapshots = read("services/cmd/gateway/central_partner_workspace_snapshots.go")
models = read("services/cmd/gateway/materialized_read_models.go")
central_reads = read("services/cmd/gateway/central_materialized_reads.go")
partner_reads = read("services/cmd/gateway/partner_materialized_reads.go")
partner_portal = read("services/cmd/gateway/partner_portal.go")
central10 = read("services/cmd/gateway/central10.go")
central13 = read("services/cmd/gateway/central13.go")
central14 = read("services/cmd/gateway/central14.go")
central17 = read("services/cmd/gateway/central17_round3.go")
readiness = read("services/cmd/gateway/read_model_readiness.go")

# Persistent, indexed Central + tenant projections.
for token in [
    "identity.central_screen_snapshots",
    "snapshot_key TEXT PRIMARY KEY",
]:
    check(token in snapshots, f"Central persistent snapshot contract missing: {token}")
for token in [
    "identity.partner_workspace_snapshots",
    "partner_id TEXT PRIMARY KEY",
    "identity.read_model_refresh_queue",
    "readModelTargetLatency = 15 * time.Millisecond",
]:
    check(token in models, f"Persistent CQRS contract missing: {token}")

# Last-Known-Good is a hard storage invariant.
check('strings.EqualFold(central10String(payload["status"]), "healthy")' in snapshots,
      "Central LKG validator no longer requires healthy status")
check("if !centralSnapshotValid(key, payload)" in snapshots,
      "Central store no longer rejects non-LKG payloads")
check("if !partnerWorkspaceSnapshotValid(payload)" in models,
      "Tenant store no longer rejects partial/unavailable payloads")
check("retaining last-known-good" in snapshots.lower(),
      "Central degraded refresh no longer documents LKG retention")
check("retaining Last-Known-Good" in models or "retaining last-known-good" in models.lower(),
      "Tenant degraded refresh no longer documents LKG retention")

# Startup must not bind the public port until every required projection exists.
for token in [
    "a.bootstrapCentralStep3Snapshots()",
    "a.bootstrapPartnerWorkspaceSnapshots()",
    "a.warmMissingCentralSnapshots()",
    "a.warmMissingCentralPartnerWorkspaces()",
    "a.ensureMaterializedReadModelsReady(readinessCtx)",
]:
    check(token in main, f"Startup read-model gate missing: {token}")
check(main.find("a.ensureMaterializedReadModelsReady(readinessCtx)") <
      main.find("common.Run(log,"),
      "Gateway binds public traffic before materialized read-model readiness")
check("ensurePartnerReadModelsReady" in readiness and "loadPartnerWorkspaceDB" in readiness,
      "Startup gate does not validate every tenant workspace")

# Read helpers may use DB + memory LKG only, never downstream services.
for signature in [
    "func (a *app) centralSnapshotForRead",
    "func (a *app) partnerWorkspaceForRead",
]:
    block = func_block(models, signature)
    check(block != "", f"Read helper missing: {signature}")
    check("internalGET" not in block and "serveProxy" not in block and "a.hosts[" not in block,
          f"{signature} regressed to downstream fan-out")
check("WHERE snapshot_key=$1" in models, "Central read path is not indexed by snapshot primary key")
check("WHERE partner_id=$1" in models, "Tenant read path is not indexed by partner primary key")

# Browser GET interception must happen before legacy owner-service proxy switches.
check(main.find("a.serveCentralMaterializedGET(w, r, u)") <
      main.find("switch {", main.find("a.serveCentralMaterializedGET(w, r, u)")),
      "Central materialized GET interceptor does not precede legacy routing")
check(partner_portal.find("a.servePartnerMaterializedGET(w,r,u,path)") <
      partner_portal.find("switch{", partner_portal.find("a.servePartnerMaterializedGET(w,r,u,path)")),
      "Partner materialized GET interceptor does not precede legacy routing")

# The materialized-serving functions themselves must be pure reads.
for source, signature, label in [
    (central_reads, "func (a *app) serveCentralMaterializedGET", "Central materialized GET"),
    (partner_reads, "func (a *app) servePartnerMaterializedGET", "Partner materialized GET"),
]:
    block = func_block(source, signature)
    check(block != "", f"{label} handler missing")
    for forbidden in ["internalGET", "internalGETWithHeaders", "serveProxy", "a.client.Do", "http.NewRequestWithContext"]:
        check(forbidden not in block, f"{label} performs forbidden request-path I/O: {forbidden}")

# Critical screen handlers must only read materialized projections.
for source, signature in [
    (central10, "func (a *app) central10Partners"),
    (central10, "func (a *app) central10Modules"),
    (central10, "func (a *app) central10Packages"),
    (central10, "func (a *app) central10Finance"),
    (central13, "func (a *app) central13Connections"),
    (central14, "func (a *app) central14Administration"),
    (central17, "func (a *app) central17Website"),
    (central17, "func (a *app) central17System"),
    (main, "func (a *app) partnerPortfolio"),
    (main, "func (a *app) partnerPortfolioMetrics"),
]:
    block = func_block(source, signature)
    check(block != "", f"Critical read handler missing: {signature}")
    for forbidden in ["internalGET", "internalGETWithHeaders", "serveProxy", "a.client.Do", "http.NewRequestWithContext"]:
        check(forbidden not in block, f"{signature} regressed to live fan-out: {forbidden}")

# Successful browser read handlers can never emit degraded/warming screen states.
for source, signature in [
    (central10, "func (a *app) central10Partners"),
    (central10, "func (a *app) central10Modules"),
    (central10, "func (a *app) central10ModulesCommercial"),
    (central10, "func (a *app) central10Packages"),
    (central10, "func (a *app) central10PackagesSupplementary"),
    (central10, "func (a *app) central10Finance"),
    (central10, "func (a *app) central10Impact"),
    (central10, "func (a *app) central10PartnerModules"),
    (central10, "func (a *app) central10PartnerWorkspace"),
    (central14, "func (a *app) central14Administration"),
    (central17, "func (a *app) central17Website"),
    (central17, "func (a *app) central17System"),
]:
    block = func_block(source, signature)
    check(block != "", f"Critical healthy-only handler missing: {signature}")
    for forbidden in ['"ready": false', '"warming"', '"partial"', '"unavailable"', 'X-Himate-Cache", "warming']:
        check(forbidden not in block, f"{signature} can expose forbidden degraded UI state: {forbidden}")

check('len(central10Step4Unavailable(raw)) > 0' in snapshots,
      "Central LKG validation does not reject hidden unavailable dependencies")
check('len(central10Step4Unavailable(raw)) > 0' in models,
      "Tenant LKG validation does not reject hidden unavailable dependencies")

# Every screen/search browser handler uses one indexed projection read.
for source, signature, read_token in [
    (central10, "func (a *app) central10Partners", "centralSnapshotForRead("),
    (central10, "func (a *app) central10Modules", "centralSnapshotForRead("),
    (central10, "func (a *app) central10ModulesCommercial", "centralSnapshotForRead("),
    (central10, "func (a *app) central10Packages", "centralSnapshotForRead("),
    (central10, "func (a *app) central10PackagesSupplementary", "centralSnapshotForRead("),
    (central10, "func (a *app) central10Finance", "centralSnapshotForRead("),
    (central10, "func (a *app) central10Impact", "centralSnapshotForRead("),
    (central10, "func (a *app) central10PartnerModules", "partnerWorkspaceForRead("),
    (central10, "func (a *app) central10PartnerWorkspace", "partnerWorkspaceForRead("),
    (central14, "func (a *app) central14Administration", "centralSnapshotForRead("),
    (central17, "func (a *app) central17Website", "centralSnapshotForRead("),
    (central17, "func (a *app) central17System", "centralSnapshotForRead("),
    (main, "func (a *app) globalSearch", "centralSnapshotForRead("),
]:
    block = func_block(source, signature)
    check(block.count(read_token) == 1,
          f"{signature} must use exactly one indexed materialized read")

check("centralStep4GlobalSearchKey" in snapshots,
      "Global search persistent read-model key is missing")
check("refreshCentralStep4GlobalSearch" in read("services/cmd/gateway/central_global_search_read_model.go"),
      "Global search background projector is missing")
compliance_fallback = func_block(main, "func (a *app) serveComplianceArchives")
check("serveComplianceMaterializedGET" in compliance_fallback and
      "http.NewRequestWithContext" not in compliance_fallback and "a.client.Do" not in compliance_fallback,
      "Compliance Archive fallback regressed to live Partners I/O")

# Partner login/access gating is a tenant projection read, not a Billing call.
access = func_block(partner_portal, "func (a *app) partnerAccessAllowed")
check("partnerWorkspaceForRead" in access and 'snapshot["portal_gate"]' in access,
      "Partner Portal access gate is not sourced from tenant LKG")
check("internalGET" not in access and 'a.hosts["billing"]' not in access,
      "Partner Portal login regressed to synchronous Billing fan-out")

# Background workers are the only place where multi-service fan-out belongs.
check("materializeCentralPartnerWorkspace" in tenant_snapshots and
      "a.internalGET(" in tenant_snapshots,
      "Tenant background materializer is missing")
for token in [
    '"catalog_modules_api"', '"provisioning_api"', '"impact_api"',
    '"evidence_api"', '"connector_credentials_api"', '"portal_gate"',
    '"tenant_finance"', '"partner_audit_events"', '"partner_contacts"',
    '"partner_domains_deployments"', '"partner_permissions"',
]:
    check(token in tenant_snapshots, f"Tenant persistent projection missing block: {token}")

for token in [
    'path == "/contacts"', 'path == "/domains"', 'path == "/deployments"',
    'path == "/audit"', 'path == "/permissions"',
]:
    check(token in partner_reads, f"Partner operational sub-screen is not materialized: {token}")

# Writes are durable + write-through before buffered response release.
check("identity.read_model_refresh_queue" in models and "enqueueReadModelRefresh" in models,
      "Durable asynchronous refresh queue missing")
check("func (a *app) writeThroughReadModels" in models,
      "Synchronous mutation write-through projection refresh missing")
check("centralStep3WaitBeginRefresh" in snapshots and
      "refreshCentralProjectionSerialized" in models and
      "writeThroughCentralPartnerWorkspace" in tenant_snapshots,
      "Write-through projections are not serialized against background refreshes")
check("deferred bool" in main and "flushDeferred" in main,
      "Mutation response buffering contract missing")
central_api = func_block(main, "func (a *app) api")
partner_api = func_block(partner_portal, "func (a *app) partnerAPI")
for block, label in [(central_api, "Central"), (partner_api, "Partner")]:
    check("writeThroughReadModels" in block, f"{label} mutation path does not perform write-through")
    check("enqueueReadModelRefresh" in block, f"{label} mutation path does not persist durable refresh event")
    check("flushDeferred" in block, f"{label} mutation response is not held until write-through completes")

if failures:
    print(f"CENTRAL-21 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-21 zero-fan-out CQRS/LKG architecture acceptance: PASS")
