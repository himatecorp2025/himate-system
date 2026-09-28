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
reports_service = read("services/cmd/reports/main.go")
central10 = read("services/cmd/gateway/central10.go")
central13 = read("services/cmd/gateway/central13.go")
central14 = read("services/cmd/gateway/central14.go")
central17 = read("services/cmd/gateway/central17_round3.go")
readiness = read("services/cmd/gateway/read_model_readiness.go")
seeds = read("services/cmd/gateway/read_model_seeds.go")
health_service = read("services/cmd/health/main.go")
common_go = read("services/internal/common/common.go")
step4_snapshots = read("services/cmd/gateway/central_step4_snapshots.go")
dashboard_snapshots = read("services/cmd/gateway/dashboard_snapshot.go")
hot_responses = read("services/cmd/gateway/central_hot_response_cache.go")

# Shared database capacity and background materializer concurrency are part of
# the CQRS contract. A zero-fan-out read path is not production-safe if the
# background workers can exhaust the shared PostgreSQL cluster.
for token in [
    'dbPoolInt("HIMATE_DB_MAX_OPEN_CONNS", 4, 1, 16)',
    'dbPoolInt("HIMATE_DB_MAX_IDLE_CONNS", 2, 0, maxOpen)',
    'db.SetMaxOpenConns(maxOpen)',
    'db.SetMaxIdleConns(maxIdle)',
    'db.SetConnMaxIdleTime(5 * time.Minute)',
]:
    check(token in common_go, f"Topology-safe PostgreSQL pool contract missing: {token}")
check(re.search(r"readModelRefreshConcurrency\s*=\s*2", models) is not None,
      "Bounded/coalesced materializer concurrency is not fixed at topology-safe value 2")
for token in [
    'readModelRefreshSlots = make(chan struct{}, readModelRefreshConcurrency)',
    'withReadModelRefreshSlot',
    'refreshReadModelsForBatch(events)',
    'WHERE processed_at IS NULL AND id <= $1',
]:
    check(token in models, f"Bounded/coalesced materializer contract missing: {token}")
check('a.refreshCentralProjectionSerialized(refresh.key, refresh.fn)' in snapshots,
      "Step3 materializer bypasses the global projection concurrency gate")
check('a.refreshCentralProjectionSerialized(refresh.key, refresh.fn)' in step4_snapshots,
      "Step4 materializer bypasses the global projection concurrency gate")
for token in [
    'centralPartnerWorkspaceMaterializeWorkers = 2',
    'centralPartnerWorkspaceSourceConcurrency  = 2',
    'centralPartnerWorkspaceGlobalWriteWorkers = 1',
    'centralPartnerWorkspaceRefreshInterval    = 60 * time.Second',
    'a.withReadModelRefreshSlot(partnerCtx, centralPartnerWorkspaceKey(partnerID)',
]:
    check(token in tenant_snapshots, f"Tenant materializer concurrency contract missing: {token}")
for token in [
    'MaxConnsPerHost:     2',
    'centralStep3RefreshInterval  = 60 * time.Second',
    'centralStep4RefreshInterval   = 60 * time.Second',
]:
    check(token in main + snapshots + step4_snapshots,
          f"Background/downstream capacity reservation contract missing: {token}")

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
check("func (a *app) buildPartnerWorkspaceLocalLKG" in models
      and "func (a *app) refreshPartnerWorkspaceLocalLKG" in models
      and 'payload["local_lkg"] = true' in models
      and "internalGET(" not in func_block(models, "func (a *app) buildPartnerWorkspaceLocalLKG"),
      "Tenant direct-route fallback is not a local zero-fan-out PostgreSQL LKG")
check("retaining last-known-good" in snapshots.lower(),
      "Central degraded refresh no longer documents LKG retention")
check("retaining Last-Known-Good" in models or "retaining last-known-good" in models.lower(),
      "Tenant degraded refresh no longer documents LKG retention")

# Startup must not bind the public port until every required projection exists.
check("var gatewayReadiness atomic.Bool" in readiness
      and "func gatewayReadinessGate" in readiness
      and "func (a *app) ensureColdStartReadiness" in readiness
      and "func (a *app) verifyColdStartLKG" in readiness,
      "Atomic two-phase startup readiness contract is incomplete")
cold_start = func_block(readiness, "func (a *app) ensureColdStartReadiness")
for token in [
    "a.seedCentralReadModelBaselines(ctx)",
    "a.bootstrapCentralStep3Snapshots()",
    "a.bootstrapPartnerWorkspaceSnapshots()",
    "a.warmMissingCentralSnapshots()",
    "a.warmMissingCentralPartnerWorkspaces()",
    "a.processReadModelRefreshQueue()",
    "a.ensureMaterializedReadModelsReady(ctx)",
    "a.verifyColdStartLKG(ctx)",
    "gatewayReadiness.Store(true)",
]:
    check(token in cold_start, f"Startup read-model gate missing: {token}")
check(cold_start.find("a.seedCentralReadModelBaselines(ctx)") <
      cold_start.find("a.bootstrapCentralStep3Snapshots()") <
      cold_start.find("a.verifyColdStartLKG(ctx)") <
      cold_start.find("gatewayReadiness.Store(true)"),
      "Cold-start phases are not ordered seed -> bootstrap -> verify -> ready")
check("a.ensureColdStartReadiness(readinessCtx)" in main
      and main.find("a.ensureColdStartReadiness(readinessCtx)") < main.find("common.Run(log,"),
      "Gateway binds public traffic before deterministic read-model readiness")
check("gatewayReadinessGate(securityHeaders(mux))" in main,
      "Public Gateway handler is not protected by the atomic readiness gate")
check("ensurePartnerReadModelsReady" in readiness and "loadPartnerWorkspaceDB" in readiness,
      "Startup gate does not validate every tenant workspace")

# CENTRAL-10..21 pre-serialized response cache: readiness cannot open until
# Central and tenant screen payloads are encoded in memory.
check("a.prewarmCentral10To21HotResponses(ctx)" in cold_start
      and cold_start.find("a.verifyColdStartLKG(ctx)") <
          cold_start.find("a.prewarmCentral10To21HotResponses(ctx)") <
          cold_start.find("gatewayReadiness.Store(true)"),
      "Cold-start order is not verify LKG -> serialize screens -> ready")
for token in [
    "type centralHotResponse struct",
    "centralHotResponseCache",
    "partnerHotResponseCache",
    "centralHotVisiblePartnerIDs",
    "tenantHotPartnerIDs",
    "buildCentralHotResponses",
    "buildPartnerHotResponses",
    "serveCentralPrewarmedResponse",
    "servePartnerPrewarmedResponse",
    "writePrewarmedResponse",
    'w.Write(entry.body)',
    'w.Header().Set("X-Himate-Cache", entry.cacheHeader)',
]:
    check(token in hot_responses, f"Serialized hot-response architecture missing: {token}")
for path in [
    '"/dashboard"', '"/company"', '"/modules"',
    '"/billing/summary"', '"/billing/subscriptions"', '"/billing/invoices"',
    '"/impact/summary"', '"/users"', '"/audit"', '"/permissions"', '"/design"',
]:
    check(path in hot_responses, f"Tenant browser prewarm surface missing: {path}")
check("partnerID: u.PartnerID, userID: u.ID" in hot_responses,
      "Tenant serialized cache is not isolated by partner and authenticated user")
check("partnerBrowserMaterializedRead(r)" in hot_responses,
      "Tenant serialized cache can bypass the explicit browser CQRS discriminator")
hot_write = func_block(hot_responses, "func writePrewarmedResponse")
check("json." not in hot_write and "w.Write(entry.body)" in hot_write,
      "Serialized live hot path performs JSON encoding or lost direct byte serving")
check('meta["duration_ms"] = 0' in hot_responses and 'meta["prewarmed"] = true' in hot_responses,
      "Prewarmed response metadata is not deterministic")
check("item := central10CopyMap(raw)" in partner_portal,
      "Partner per-user prewarm can mutate shared tenant LKG module maps")
check(main.find("a.serveCentralPrewarmedResponse(w, r, u)") <
      main.find("a.serveCentralMaterializedGET(w, r, u)"),
      "Central serialized hot path does not precede normal materialized dispatch")
check(partner_portal.find("a.servePartnerPrewarmedResponse(w,r,u,path)") <
      partner_portal.find("a.servePartnerMaterializedGET(w,r,u,path)"),
      "Tenant serialized hot path does not precede normal materialized dispatch")
check("invalidateCentralHotResponseCaches()" in models
      and models.count("a.requestCentralHotResponseRefresh()") >= 2,
      "Foreground/durable reconciliation does not restore serialized hot responses")

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
check("a.seedCentralReadModelBaselines(ctx)" in cold_start
      and cold_start.find("a.seedCentralReadModelBaselines(ctx)") < cold_start.find("a.bootstrapCentralStep3Snapshots()"),
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

# Browser read helpers are committed-memory-first LKG caches with exactly one
# indexed PostgreSQL recovery path and no downstream service fan-out.
central_read = func_block(models, "func (a *app) centralSnapshotForRead")
partner_read = func_block(models, "func (a *app) partnerWorkspaceForRead")
for block, signature in [
    (central_read, "centralSnapshotForRead"),
    (partner_read, "partnerWorkspaceForRead"),
]:
    check(block != "", f"Read helper missing: {signature}")
    check("internalGET" not in block and "serveProxy" not in block and "a.hosts[" not in block,
          f"{signature} regressed to downstream fan-out")
check(central_read.find("centralStep3SnapshotGet(key)") >= 0
      and central_read.find("centralStep3SnapshotGet(key)") < central_read.find("a.loadCentralSnapshotDB(ctx, key)"),
      "Central browser read path is not memory-LKG first")
check(partner_read.find("centralStep3SnapshotGet(key)") >= 0
      and partner_read.find("centralStep3SnapshotGet(key)") < partner_read.find("a.loadPartnerWorkspaceDB(ctx, partnerID)"),
      "Tenant browser read path is not memory-LKG first")
check("WHERE snapshot_key=$1" in models, "Central DB recovery path is not indexed by snapshot primary key")
check("WHERE partner_id=$1" in models, "Tenant DB recovery path is not indexed by partner primary key")

dashboard_read = func_block(dashboard_snapshots, "func (a *app) dashboardSnapshotForReadContext")
check("dashboardSnapshotValid(memory)" in dashboard_read
      and dashboard_read.find("dashboardSnapshotValid(memory)") < dashboard_read.find("a.loadDashboardSnapshotContext(ctx, year)"),
      "Current-year dashboard browser read path is not memory-LKG first")
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
write_through = func_block(models, "func (a *app) writeThroughReadModels")
check("if scope.system {" in write_through
      and "a.refreshHealthSourceWriteThrough()" in write_through
      and "context.WithTimeout(context.Background(), 3*time.Second)" in models,
      "Health source write-through is unbounded or not isolated to System mutations")
check("func classifyReadModelMutation" in models
      and 'strings.Contains(path, "/portal-users")' in models
      and 'strings.HasPrefix(path, "/partner/api/v1/")' in models
      and 'partnerMutation := strings.Contains(reason, "partner")' not in models,
      "Partner mutation scope is still substring-based and can trigger projection storms")
check('case path == "/api/v1/partner-categories":' in central_reads and
      'centralSnapshotForRead(r.Context(), centralStep4PartnersKey)' in central_reads,
      "Partner category GET is not routed through the persistent Partners read model")

# Browser GET interception must happen before legacy owner-service proxy switches.
check(main.find("a.serveCentralMaterializedGET(w, r, u)") <
      main.find("switch {", main.find("a.serveCentralMaterializedGET(w, r, u)")),
      "Central materialized GET interceptor does not precede legacy routing")
check("func centralBrowserMaterializedRead" in central_reads and
      'r.Header.Get("X-Himate-Locale")' in central_reads,
      "Browser/legacy compatibility discriminator is missing")
central_materialized = func_block(central_reads, "func (a *app) serveCentralMaterializedGET")
check("centralBrowserMaterializedRead(r)" in central_materialized,
      "Central materialized GET no longer requires browser read-model identity")
check('case r.URL.Path == "/api/v1/partners", r.URL.Path == "/api/v1/partner-categories":' in main and
      'case r.URL.Path == "/api/v1/partners" && r.Method == http.MethodGet:' not in main,
      "Legacy Partners GET is still intercepted by a stale materialized compatibility handler")
check('case r.URL.Path == "/api/v1/environments", strings.HasPrefix(r.URL.Path, "/api/v1/environments/"):' in main and
      'a.serveProxy(w, r, "environments")' in main,
      "Legacy environment compatibility path no longer reaches the authoritative environment service")
check('case strings.HasPrefix(r.URL.Path, "/api/v1/system-health"):' in main and
      'a.serveSystemHealthCompatibility(w, r)' in main,
      "Legacy system-health compatibility path is not routed through local persistent LKG")
health_compat = func_block(models, "func (a *app) serveSystemHealthCompatibility")
check("centralStep4SystemKey" in health_compat
      and 'step4Map(snapshot["health_api"])' in health_compat
      and "internalGET" not in health_compat
      and "serveProxy" not in health_compat,
      "Legacy system-health compatibility path can still perform request-time fan-out")
gateway_health = func_block(models, "func (a *app) serveGatewayHealthCompatibility")
check('mux.HandleFunc("/health", a.serveGatewayHealthCompatibility)' in main
      and 'mux.HandleFunc("/api/v1/health", a.serveGatewayHealthCompatibility)' in main
      and "centralSnapshotForRead" in gateway_health,
      "Gateway health compatibility routes are not local LKG reads")
for forbidden in ["internalGET", "serveProxy", "a.client.Do", "http.NewRequest"]:
    check(forbidden not in gateway_health,
          f"Gateway health compatibility regressed to request fan-out: {forbidden}")
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

# Connections is a deliberate dual-path compatibility exception. Browser reads
# remain persistent zero-fan-out, while non-browser /api/v1 compatibility reads
# may rebuild from authoritative committed sources for historical/runtime smoke.
connections_dispatch = func_block(central13, "func (a *app) central13Connections")
connections_browser = func_block(central13, "func (a *app) serveCentral13PersistentConnections")
connections_legacy = func_block(central13, "func (a *app) serveLegacyCentral13Connections")
check("centralBrowserMaterializedRead(r)" in connections_dispatch
      and "serveCentral13PersistentConnections" in connections_dispatch
      and "serveLegacyCentral13Connections" in connections_dispatch,
      "Connections route does not explicitly split browser CQRS and legacy compatibility reads")
check("centralSnapshotForRead" in connections_browser
      and "materializeCentralConnections" not in connections_browser,
      "Connections browser path is not a pure persistent read-model read")
check("materializeCentralConnections" in connections_legacy
      and "central10Cached" in connections_legacy
      and "central10Store" in connections_legacy,
      "Connections legacy compatibility path is not authoritative and cache-bounded")

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

# Shipped-browser Partner Portal GETs are LKG-first. Authentication, session
# checks, legacy compatibility reads and mutations use committed local authority
# so activation/suspension transitions are visible immediately without fan-out.
access = func_block(partner_portal, "func (a *app) partnerAccessAllowed")
access_snapshot = func_block(partner_portal, "func (a *app) partnerAccessSnapshot")
request_access = func_block(partner_portal, "func (a *app) partnerRequestAccess")
authoritative_access = func_block(partner_portal, "func (a *app) partnerAuthoritativeAccessAllowed")
check("partnerWorkspaceForRead" in access_snapshot and
      'snapshot["portal_gate"]' in access_snapshot and
      "internalGET" not in access_snapshot,
      "Partner request access gate is not sourced from tenant LKG")
check("partnerAccessSnapshot(ctx, partnerID)" in request_access
      and "partnerAuthoritativeAccessAllowed(ctx, partnerID)" in request_access
      and "errors.Is(err, errPartnerPortalAccessDisabled)" in request_access,
      "Partner request access does not use fail-closed LKG-first compatibility fallback")
check("partnerAuthoritativeAccessAllowed" in access
      and "partners.partners" in authoritative_access
      and "billing.partner_onboarding" in authoritative_access
      and "QueryRowContext" in authoritative_access
      and "internalGET" not in authoritative_access,
      "Partner authoritative fallback is not a local committed projection")
partner_login = func_block(partner_portal, "func (a *app) partnerLogin")
check("partnerAccessAllowed(ctx,u.PartnerID)" in partner_login
      and "partnerRequestAccess(ctx,u.PartnerID)" not in partner_login,
      "Partner login is not using committed local access authority")
partner_me = func_block(partner_portal, "func (a *app) partnerMe")
check("partnerAccessAllowed(ctx,u.PartnerID)" in partner_me
      and "partnerRequestAccess(ctx,u.PartnerID)" not in partner_me,
      "Partner session check is not using committed local access authority")
partner_api_access = func_block(partner_portal, "func (a *app) partnerAPI")
check("partnerBrowserMaterializedRead(r)" in partner_api_access
      and "partnerRequestAccess(accessCtx,u.PartnerID)" in partner_api_access
      and "partnerAccessAllowed(accessCtx,u.PartnerID)" in partner_api_access,
      "Partner API does not split browser LKG reads from authoritative compatibility/mutations")
check("partnerWorkspaceContextKey" in partner_portal and
      "context.WithValue" in partner_api_access and
      "r.Context().Value(partnerWorkspaceContextKey{})" in partner_portal,
      "Partner request does not reuse the access-gate tenant snapshot")

tenant_scope = func_block(models, "func readModelTenantSliceScopes")
tenant_slice = func_block(tenant_snapshots, "func (a *app) refreshGlobalTenantReadModelSlice")
check('strings.Contains(path, "/onboarding")' in tenant_scope
      and 'strings.Contains(path, "/invoice")' in tenant_scope
      and "access, billing bool" in tenant_scope,
      "Tenant slice classifier does not cover immediate access/Billing consistency")
for token in [
    'fetch("partner_access", "partners"',
    'fetch("portal_gate", "billing"',
    'fetch("billing_summary", "billing"',
    'fetch("portal_invoices", "billing"',
    'snapshot["partner"] = partner',
    'snapshot["portal_gate"] = portalGate',
    'snapshot["billing"] = billingSummary',
    'snapshot["portal_billing_invoices"] = portalInvoices',
]:
    check(token in tenant_slice, f"Tenant synchronous access/Billing slice missing: {token}")

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

# Source-owned background writes participate in the same durable CQRS event
# stream. Report READY/evidence linkage must not become visible until the
# corresponding Impact + tenant projections have consumed the event.
for token in [
    "identity.read_model_refresh_queue",
    "/internal/reports/background-complete/evidence/report",
    "processed_at FROM identity.read_model_refresh_queue",
    "synchronizeReadModels(ctx,partnerIDs)",
    "requeueProjectionSync",
]:
    check(token in reports_service, f"Reports background CQRS event contract missing: {token}")
check("targetKeys := map[string]bool{}" in models and
      "readModelVerificationKeys(reason)" in models and
      "jobsByKey := map[string]func(){}" in models,
      "Durable refresh queue does not coalesce events into targeted projections")
check("for key := range targetKeys" in models,
      "Durable refresh verification is not scoped to affected projections")

# Writes are durable + synchronous write-through before the buffered mutation
# response is released. The durable worker is deliberately not woken from the
# request path; it reconciles after the mutation quiet window.
check("identity.read_model_refresh_queue" in models
      and "persistReadModelRefreshEvent" in models
      and "stageReadModelRefresh" in models,
      "Durable staged refresh queue missing")
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
    stage = block.find("stageReadModelRefresh")
    write = block.find("writeThroughReadModels", stage)
    check(stage >= 0 and stage < write,
          f"{label} mutation path is not ordered durable-stage -> synchronous write-through")
    check("wakeReadModelRefreshWorker()" not in block,
          f"{label} mutation path can still wake durable reconciliation before ACK")
    check("flushDeferred" in block,
          f"{label} mutation response is not held until write-through completes")

check(central_api.find("finalizeAuditIntent") < central_api.find("writeThroughReadModels"),
      "Central mutation projection refresh runs before durable audit finalization")
check('refreshReason += "/audit"' in central_api,
      "Central mutation write-through does not include the newly finalized audit projection")
check("if scope.system {" in write_through
      and 'strings.Contains(foregroundReason, "environment") || strings.Contains(foregroundReason, "provision")' in write_through
      and 'add(centralStep4WebsiteKey, a.refreshCentralStep4Website)' in write_through,
      "Provisioning/environment write-through does not refresh the browser Website environment projection")

# Provider/webhook writes must participate in the same write-through contract.
check('"partner_id":x.PartnerID' in payments,
      "Payment webhook acknowledgement no longer identifies the settled tenant")

connector_proxy = func_block(main, "func (a *app) connectorPublicProxy")
check(connector_proxy != "", "Gateway connector ingestion projection bridge is missing")
check('"partner_id"' in connector_proxy and 'reason += "/impact"' in connector_proxy,
      "Connector write-through bridge does not scope tenant/impact projection refreshes")
check('mux.HandleFunc("/connector/v1/", a.connectorPublicProxy)' in main,
      "Connector public writes bypass the CQRS write-through bridge")

public_contact = func_block(main, "func (a *app) publicContact")
check(public_contact != "", "Public Contact projection bridge is missing")
check("deferred: true" in public_contact and "flushDeferred" in public_contact,
      "Public Contact write does not buffer the ACK through synchronous projection refresh")

webhook_proxy = func_block(main, "func (a *app) stripeWebhookProxy")
check(webhook_proxy != "", "Gateway payment webhook projection bridge is missing")
check('mux.HandleFunc("/webhooks/stripe", a.stripeWebhookProxy)' in main,
      "Stripe webhook bypasses the CQRS write-through bridge")

for block, label in [
    (connector_proxy, "Connector ingestion"),
    (public_contact, "Public Contact"),
    (webhook_proxy, "Payment settlement"),
]:
    stage = block.find("stageReadModelRefresh")
    write = block.find("writeThroughReadModels", stage)
    check(stage >= 0 and stage < write,
          f"{label} is not ordered durable-stage -> synchronous write-through")
    check("wakeReadModelRefreshWorker()" not in block,
          f"{label} can still wake durable reconciliation before ACK")
check("readModelBatchShouldDefer" in models
      and "readModelMutationQuiet" in models
      and "readModelMutationMaxDeferral" in models,
      "Durable queue is not protected by a general mutation quiet window")
check("requestCentralStep3Refresh()" not in central10
      and "requestCentralStep4Refresh()" not in central10
      and "requestAllCentralPartnerWorkspaceRefreshes()" not in central10,
      "Central cache invalidation still starts a duplicate materialization pipeline")

if failures:
    print(f"CENTRAL-21 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-21 zero-fan-out CQRS/LKG architecture acceptance: PASS")
