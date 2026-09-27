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
catalog = read("services/cmd/catalog/main.go")
models = read("services/cmd/gateway/materialized_read_models.go")
central_reads = read("services/cmd/gateway/central_materialized_reads.go")
partner_reads = read("services/cmd/gateway/partner_materialized_reads.go")
partner_portal = read("services/cmd/gateway/partner_portal.go")
payments = read("services/cmd/payments/main.go")
central10 = read("services/cmd/gateway/central10.go")
central13 = read("services/cmd/gateway/central13.go")
central14 = read("services/cmd/gateway/central14.go")
central17 = read("services/cmd/gateway/central17_round3.go")
readiness = read("services/cmd/gateway/read_model_readiness.go")
seeds = read("services/cmd/gateway/read_model_seeds.go")
health_service = read("services/cmd/health/main.go")

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
]:
    check(token in models, f"Persistent CQRS contract missing: {token}")
check("readModelTargetLatency" in models and "15 * time.Millisecond" in models,
      "Persistent CQRS contract missing: readModelTargetLatency=15ms")
check("centralReadModelBaselines()[key]" in models,
      "Central read path has no healthy structural fallback after DB+memory LKG failure")
dashboard_handler = func_block(main, "func (a *app) dashboard(w http.ResponseWriter")
check("dashboardReadModelBaseline" in dashboard_handler and "dashboardWarmingSnapshot" not in dashboard_handler,
      "Dashboard browser read path can still emit a warming/degraded fallback")

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
    "a.processReadModelRefreshQueue()",
    "a.ensureMaterializedReadModelsReady(readinessCtx)",
]:
    check(token in main, f"Startup read-model gate missing: {token}")
check(main.find("a.ensureMaterializedReadModelsReady(readinessCtx)") <
      main.find("common.Run(log,"),
      "Gateway binds public traffic before materialized read-model readiness")
check(main.find("a.processReadModelRefreshQueue()") <
      main.find("a.ensureMaterializedReadModelsReady(readinessCtx)"),
      "Durable projection events are not replayed before the startup readiness gate")
check("ensurePartnerReadModelsReady" in readiness and "loadPartnerWorkspaceDB" in readiness,
      "Startup gate does not validate every tenant workspace")

# Cold start always has a DB-backed healthy structural baseline, while startup
# still attempts to replace seeded rows with real projections before bind.
check("seedCentralReadModelBaselines" in seeds,
      "Cold-start Central/Dashboard baseline seeder missing")
check("seedPartnerWorkspaceBaseline" in seeds,
      "Cold-start tenant workspace baseline seeder missing")
check("centralSnapshotValid(key, existing)" in seeds and
      "partnerWorkspaceSnapshotValid(existing)" in seeds,
      "Baseline repair does not protect existing Last-Known-Good projections")
check("err != sql.ErrNoRows" in seeds and
      "ON CONFLICT(snapshot_key) DO UPDATE" in seeds and
      "ON CONFLICT(partner_id) DO UPDATE" in seeds,
      "Cold-start baseline repair contract missing for absent/corrupt projections")
check("a.seedCentralReadModelBaselines(ctx)" in main and
      main.find("a.seedCentralReadModelBaselines(ctx)") < main.find("a.bootstrapCentralStep3Snapshots()"),
      "Cold-start baseline seeding does not happen before snapshot bootstrap")
for token in [
    "centralStep3RegistryKey", "centralStep3PlansKey", "centralStep3AnalyticsKey",
    "centralStep3CommercialKey", "centralStep4PartnersKey", "centralStep4FinanceKey",
    "centralStep4ImpactKey", "centralStep4AdministrationKey", "centralStep4SystemKey",
    "centralStep4WebsiteKey", "centralStep4ConnectionsKey", "centralStep4ComplianceKey",
    "centralStep4GlobalSearchKey",
]:
    check(token in seeds, f"Cold-start Central baseline coverage missing: {token}")
check('payload["seeded"] == true' in seeds,
      "Seeded read-model marker missing")
check("readModelSeeded(payload)" in readiness and "job.refresh()" in readiness,
      "Startup readiness can accept a seeded Central row without attempting real materialization")
check("seedPartnerWorkspaceBaseline(ctx, item)" in readiness,
      "Startup readiness does not seed missing tenant workspaces before materialization")

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
check('path == "/api/v1/system-health"' in central_reads and
      'centralSnapshotForRead(r.Context(), centralStep4SystemKey)' in central_reads,
      "Legacy system-health GET is not routed through the persistent System read model")
check('mux.HandleFunc("/api/v1/system-health",a.systemHealthSnapshot)' in health_service,
      "Health compatibility GET no longer serves its persistent snapshot")
check('mux.HandleFunc("/internal/v1/system-health/refresh",a.systemHealthRefresh)' in health_service,
      "Health background/write-through refresh endpoint is missing")
health_snapshot_handler = func_block(health_service, "func (a *app)systemHealthSnapshot")
for forbidden in ["checkServices(", "partnerHealth(", "http.NewRequest", "a.client.Do("]:
    check(forbidden not in health_snapshot_handler,
          f"Health snapshot GET regressed to live fan-out: {forbidden}")
health_refresh_handler = func_block(health_service, "func (a *app)systemHealthRefresh")
check("refreshSystemHealthSnapshots" in health_refresh_handler,
      "Health write-through refresh no longer rebuilds persisted health state")
check("refreshHealthSourceWriteThrough" in models and
      "a.refreshHealthSourceWriteThrough()" in models,
      "Gateway write-through does not refresh the health source projection before System CQRS rebuild")
check('case path == "/api/v1/partner-categories":' in central_reads and
      'centralSnapshotForRead(r.Context(), centralStep4PartnersKey)' in central_reads,
      "Partner category GET is not routed through the persistent Partners read model")

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

# Central routing must not short-circuit persistent reads through the legacy
# process-memory response cache.
central_router = func_block(central10, "func (a *app) central10ReadModel")
check("central10Cached(" not in central_router,
      "Central browser read router can bypass the persistent DB projection through legacy response cache")

# Critical screen handlers must only read materialized projections. A browser
# GET may not even *schedule* a refresh; projection freshness belongs to
# periodic workers, durable events and mutation write-through.
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
    (central13, "func (a *app) central13Connections"),
    (central14, "func (a *app) central14Administration"),
    (central17, "func (a *app) central17Website"),
    (central17, "func (a *app) central17System"),
    (main, "func (a *app) dashboard(w http.ResponseWriter"),
    (main, "func (a *app) partnerPortfolio"),
    (main, "func (a *app) partnerPortfolioMetrics"),
]:
    block = func_block(source, signature)
    check(block != "", f"Critical read handler missing: {signature}")
    for forbidden in [
        "internalGET", "internalGETWithHeaders", "serveProxy", "a.client.Do",
        "http.NewRequestWithContext", "requestDashboardRefresh(",
        "requestCentralStep3Refresh(", "requestCentralStep4Refresh(",
        "requestCentralPartnerWorkspaceRefresh(",
        "requestAllCentralPartnerWorkspaceRefreshes(",
    ]:
        check(forbidden not in block, f"{signature} regressed to read-triggered I/O: {forbidden}")

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
access_snapshot = func_block(partner_portal, "func (a *app) partnerAccessSnapshot")
check("partnerAccessSnapshot" in access and
      "partnerWorkspaceForRead" in access_snapshot and
      'snapshot["portal_gate"]' in access_snapshot,
      "Partner Portal access gate is not sourced from tenant LKG")
for block in [access, access_snapshot]:
    check("internalGET" not in block and 'a.hosts["billing"]' not in block,
          "Partner Portal login regressed to synchronous Billing fan-out")
partner_api_access = func_block(partner_portal, "func (a *app) partnerAPI")
check("partnerWorkspaceContextKey" in partner_portal and
      "context.WithValue" in partner_api_access and
      "r.Context().Value(partnerWorkspaceContextKey{})" in partner_portal,
      "Partner Portal request does not reuse the access-gate tenant snapshot")

# Remaining deep screen reads must also be projected; none may fall through
# to Catalog/Connector/Partner live proxies.
for token in [
    '"/internal/v1/read-model/module-details"',
    '"/internal/v1/read-model/partner-module-history/"',
]:
    check(token in catalog, f"Catalog bulk projection source missing: {token}")

for token in [
    '"module_details"',
]:
    check(token in snapshots, f"Central module detail LKG field missing: {token}")
for token in [
    '"module_commercial_history"',
    '"start22_summary"',
    '"start22_retention"',
]:
    check(token in models and token in tenant_snapshots,
          f"Tenant projection field missing or not validated: {token}")

for token in [
    'case "relationships":',
    'case "impact-metrics":',
    'case "usage":',
    'strings.HasSuffix(path, "/commercial-history")',
    'path == "/api/v1/connectors/start22/mapping"',
    'path == "/api/v1/connectors/start22/summary"',
    'path == "/api/v1/connectors/start22/retention"',
]:
    check(token in central_reads, f"Deep materialized GET route missing: {token}")

check('"start22_mapping"' in snapshots and '"start22_summary"' in snapshots,
      "Connections LKG validator does not require complete START-22 projection")

# Background workers are the only place where multi-service fan-out belongs.
check("materializeCentralPartnerWorkspace" in tenant_snapshots and
      "a.internalGET(" in tenant_snapshots,
      "Tenant background materializer is missing")
for token in [
    '"catalog_modules_api"', '"provisioning_api"', '"impact_api"',
    '"evidence_api"', '"connector_credentials_api"', '"portal_gate"',
    '"tenant_finance"', '"partner_audit_events"', '"partner_contacts"',
    '"partner_domains_deployments"', '"partner_permissions"', '"company_profile"',
]:
    check(token in tenant_snapshots, f"Tenant persistent projection missing block: {token}")

for token in [
    'path == "/contacts"', 'path == "/domains"', 'path == "/deployments"',
    'path == "/audit"', 'path == "/permissions"',
]:
    check(token in partner_reads, f"Partner operational sub-screen is not materialized: {token}")

for token in [
    '"backup_restore_points"', '"backup_restore_tests_api"', '"backup_restore_jobs_api"',
]:
    check(token in read("services/cmd/gateway/central17_round3.go"),
          f"System recovery projection missing block: {token}")
    check(token in snapshots, f"System LKG validator missing recovery block: {token}")

for token in [
    'path == "/api/v1/backups/restore-tests"',
    'strings.HasPrefix(path, "/api/v1/backups/restore-tests/")',
    'path == "/api/v1/backups/restores"',
    'strings.HasPrefix(path, "/api/v1/backups/restores/")',
    'strings.HasPrefix(path, "/api/v1/backups/restore-points/")',
]:
    check(token in central_reads, f"Backup recovery browser GET is not materialized: {token}")

# Binary reads are materialized too: Partner invoice PDF must use one tenant LKG.
check('"company_profile"' in tenant_snapshots and '"company_profile"' in models,
      "Tenant LKG does not carry the invoice issuer/company profile")
check('strings.HasPrefix(path, "/billing/invoices/") && strings.HasSuffix(path, "/pdf")' in partner_reads,
      "Partner invoice PDF GET is not intercepted by the tenant read model")
pdf_read = func_block(partner_reads, "func servePartnerInvoicePDFFromSnapshot")
check(pdf_read != "", "Materialized Partner invoice PDF renderer is missing")
for forbidden in ["internalGET", "internalGETWithHeaders", "serveProxy", "a.client.Do", "http.NewRequestWithContext"]:
    check(forbidden not in pdf_read, f"Partner invoice PDF regressed to live fan-out: {forbidden}")

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

# Provider/webhook writes must participate in the same write-through contract.
check('"partner_id":x.PartnerID' in payments,
      "Payment webhook acknowledgement no longer identifies the settled tenant")

connector_proxy = func_block(main, "func (a *app) connectorPublicProxy")
check(connector_proxy != "", "Gateway connector ingestion projection bridge is missing")
check("enqueueReadModelRefresh" in connector_proxy and "writeThroughReadModels" in connector_proxy,
      "Connector ingestion does not synchronously refresh durable tenant/Central projections")
check('"partner_id"' in connector_proxy and 'reason += "/impact"' in connector_proxy,
      "Connector write-through bridge does not scope tenant/impact projection refreshes")
check('mux.HandleFunc("/connector/v1/", a.connectorPublicProxy)' in main,
      "Connector public writes bypass the CQRS write-through bridge")

public_contact = func_block(main, "func (a *app) publicContact")
check(public_contact != "", "Public Contact projection bridge is missing")
check("deferred: true" in public_contact and "enqueueReadModelRefresh" in public_contact and
      "writeThroughReadModels" in public_contact and "flushDeferred" in public_contact,
      "Public Contact write does not synchronously refresh Website read models before ACK")

webhook_proxy = func_block(main, "func (a *app) stripeWebhookProxy")
check(webhook_proxy != "", "Gateway payment webhook projection bridge is missing")
check("enqueueReadModelRefresh" in webhook_proxy and "writeThroughReadModels" in webhook_proxy,
      "Payment settlement does not synchronously refresh durable tenant/Central projections")
check('mux.HandleFunc("/webhooks/stripe", a.stripeWebhookProxy)' in main,
      "Stripe webhook bypasses the CQRS write-through bridge")

if failures:
    print(f"CENTRAL-21 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-21 zero-fan-out CQRS/LKG architecture acceptance: PASS")
