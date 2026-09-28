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
partner_reads = read("services/cmd/gateway/partner_materialized_reads.go")
partner_portal = read("services/cmd/gateway/partner_portal.go")
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
      "Generic Central materialized GET interceptor can still capture legacy/smoke reads")
check("func partnerBrowserMaterializedRead" in partner_reads
      and 'r.Header.Get("X-Himate-Read-Model")' in partner_reads
      and '"browser"' in partner_reads
      and 'r.Header.Get("X-Himate-Locale")' in partner_reads,
      "Partner Portal browser CQRS discriminator is missing or implicit")
check("if !partnerBrowserMaterializedRead(r)" in partner_reads,
      "Partner Portal materialized GET interceptor can still capture legacy/smoke reads")
partner_api_start = partner_portal.find("func (a *app) partnerAPI")
partner_api_end = partner_portal.find("\nfunc ", partner_api_start + 1)
partner_api_block = partner_portal[partner_api_start:partner_api_end if partner_api_end > partner_api_start else len(partner_portal)]
check("func (a *app) partnerRequestAccess" in partner_portal
      and "partnerAccessSnapshot(ctx, partnerID)" in partner_portal
      and "partnerAuthoritativeAccessAllowed(ctx, partnerID)" in partner_portal,
      "Partner Portal LKG-first access compatibility adapter is missing")
auth_start = partner_portal.find("func (a *app) partnerAuthoritativeAccessAllowed")
auth_end = partner_portal.find("\nfunc ", auth_start + 1)
auth_block = partner_portal[auth_start:auth_end if auth_end > auth_start else len(partner_portal)]
check(auth_start >= 0
      and "partners.partners" in auth_block
      and "billing.partner_onboarding" in auth_block
      and "QueryRowContext" in auth_block
      and "internalGET" not in auth_block,
      "Partner login compatibility fallback still performs request-path service fan-out")
check("partnerRequestAccess(accessCtx,u.PartnerID)" in partner_api_block
      and "partnerAccessAllowed(accessCtx,u.PartnerID)" not in partner_api_block,
      "Authenticated Partner Portal request path can still block on live access fan-out")
partner_login_start = partner_portal.find("func (a *app) partnerLogin")
partner_login_end = partner_portal.find("\nfunc ", partner_login_start + 1)
partner_login_block = partner_portal[partner_login_start:partner_login_end if partner_login_end > partner_login_start else len(partner_portal)]
check("partnerRequestAccess(ctx,u.PartnerID)" in partner_login_block
      and "partnerAccessAllowed(ctx,u.PartnerID)" not in partner_login_block,
      "Partner login can still block on live access fan-out")
check('case r.URL.Path == "/api/v1/environments", strings.HasPrefix(r.URL.Path, "/api/v1/environments/"):' in main
      and 'a.serveProxy(w, r, "environments")' in main,
      "Legacy environment GET no longer reaches authoritative Environment service")
check('case strings.HasPrefix(r.URL.Path, "/api/v1/provisioning/"):' in main
      and 'a.serveProxy(w, r, "provisioning")' in main,
      "Legacy provisioning GET no longer reaches authoritative Provisioning service")
check('case strings.HasPrefix(r.URL.Path, "/api/v1/system-health"):' in main
      and 'a.serveSystemHealthCompatibility(w, r)' in main,
      "Legacy system-health GET is not routed through the local LKG compatibility adapter")
health_compat_start = models.find("func (a *app) serveSystemHealthCompatibility")
health_compat_end = models.find("\nfunc ", health_compat_start + 1)
health_compat = models[health_compat_start:health_compat_end if health_compat_end > health_compat_start else len(models)]
check(health_compat_start >= 0
      and "centralStep4SystemKey" in health_compat
      and 'step4Map(snapshot["health_api"])' in health_compat
      and "internalGET(" not in health_compat
      and "serveProxy(" not in health_compat,
      "Legacy system-health compatibility path can still fan out or bypass persistent System LKG")
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

# Atomic two-phase cold-start readiness contract.
check("var gatewayReadiness atomic.Bool" in readiness
      and "func gatewayReadinessGate" in readiness
      and "func (a *app) ensureColdStartReadiness" in readiness
      and "func (a *app) verifyColdStartLKG" in readiness,
      "Atomic cold-start readiness gate is incomplete")
ensure_start = readiness.find("func (a *app) ensureColdStartReadiness")
ensure_block = readiness[ensure_start:]
check(ensure_start >= 0
      and ensure_block.find("seedCentralReadModelBaselines") >= 0
      and ensure_block.find("verifyColdStartLKG") >= 0
      and ensure_block.find("verifyColdStartLKG") < ensure_block.find("gatewayReadiness.Store(true)"),
      "Readiness can open before deterministic seed/LKG verification")
check("gatewayReadinessGate(securityHeaders(mux))" in main,
      "Gateway public handler is not protected by the atomic readiness gate")
check('path == "/healthz"' in readiness
      and 'path == "/api/v1/live"' not in readiness[readiness.find("func gatewayReadinessGate"):readiness.find("\nfunc ", readiness.find("func gatewayReadinessGate") + 1)],
      "Readiness gate exposes a non-healthz route before deterministic LKG readiness")
check('mux.HandleFunc("/api/v1/health", a.serveGatewayHealthCompatibility)' in main
      and 'mux.HandleFunc("/health", a.serveGatewayHealthCompatibility)' in main,
      "Gateway health compatibility routes still use live request-path fan-out")
gateway_health_start = models.find("func (a *app) serveGatewayHealthCompatibility")
gateway_health_end = models.find("\nfunc ", gateway_health_start + 1)
gateway_health = models[gateway_health_start:gateway_health_end if gateway_health_end > gateway_health_start else len(models)]
check(gateway_health_start >= 0
      and "centralStep4SystemKey" in gateway_health
      and "service_versions" in gateway_health
      and "release_consistent" in gateway_health
      and "internalGET(" not in gateway_health
      and "serveProxy(" not in gateway_health,
      "Gateway release-health compatibility path can still fan out or lose release contract fields")

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
# Global definitions are cross-tenant, but foreground mutation latency must not
# scale with tenant count. The concrete tenant/Central slices are synchronous;
# global tenant reconciliation belongs to the durable queue.
check("func readModelGlobalTenantScopes" in models
      and 'path == "/api/v1/modules"' in models
      and 'path == "/api/v1/module-groups"' in models
      and 'path == "/api/v1/billing/plans"' in models
      and 'path == "/api/v1/cms/design"' in models,
      "Global tenant mutation classifier is incomplete")
write_through_start = models.find("func (a *app) writeThroughReadModels")
write_through_end = models.find("\nfunc ", write_through_start + 1)
write_through = models[write_through_start:write_through_end if write_through_end > write_through_start else len(models)]
check(write_through_start >= 0
      and "writeThroughGlobalTenantReadModels(reason)" not in write_through,
      "Foreground mutation path still performs unbounded all-tenant fan-out")
check("writeThroughGlobalTenantReadModels" in workspace,
      "Targeted global tenant reconciliation implementation disappeared")
check("refreshGlobalTenantReadModelSlice" in workspace
      and "/internal/v1/partner-portal/" in workspace
      and "/api/v1/billing/plans" in workspace
      and "/internal/v1/cms/partner-design/" in workspace,
      "Global tenant write-through does not rebuild module/plan/design tenant slices")
check('strings.Contains(reason, "/modules")' not in models,
      "All-tenant refresh classifier is still broad enough to capture partner-scoped module writes")
check("writeThroughGlobalTenantReadModelScopes" in workspace
      and '"durable-read-model-batch"' in models,
      "Durable queue does not reuse targeted global tenant slice reconciliation")
api_start = main.find("func (a *app) api")
api_end = main.find("\nfunc ", api_start + 1)
api_block = main[api_start:api_end if api_end > api_start else len(main)]
api_stage = api_block.find("a.stageReadModelRefresh")
api_write = api_block.find("a.writeThroughReadModels", api_stage)
check("stageReadModelRefresh" in models
      and api_stage >= 0 and api_stage < api_write
      and "wakeReadModelRefreshWorker()" not in api_block,
      "Admin mutation path can still race durable reconciliation against synchronous write-through")
partner_stage = partner_api_block.find("a.stageReadModelRefresh")
partner_write = partner_api_block.find("a.writeThroughReadModels", partner_stage)
check(partner_stage >= 0 and partner_stage < partner_write
      and "wakeReadModelRefreshWorker()" not in partner_api_block,
      "Partner mutation path can still race durable reconciliation against synchronous write-through")
global_slice_start = workspace.find("func (a *app) refreshGlobalTenantReadModelSlice")
global_slice_end = workspace.find("\nfunc ", global_slice_start + 1)
global_slice = workspace[global_slice_start:global_slice_end if global_slice_end > global_slice_start else len(workspace)]
check('if moduleScope {' in global_slice
      and 'fetch("portal_plans", "billing"' not in global_slice.split("if planScope {", 1)[0]
      and 'fetch("portal_plan", "billing"' not in global_slice.split("if planScope {", 1)[0],
      "Global module write-through still refetches unchanged Billing plan state per tenant")
check("read-model refresh batch remains pending" not in models
      and "read-model refresh batch consumed after bounded LKG reconciliation attempt" in models,
      "Durable read-model queue can still head-of-line replay a failed batch forever")
check("readModelRefreshAcquireBudget  = 8 * time.Second" in models
      or "readModelRefreshAcquireBudget = 8 * time.Second" in models,
      "Foreground read-model lock acquisition is no longer bounded to the accepted budget")
check("foregroundModuleDeltaMarker" in models
      and "readModelForegroundModuleDelta" in models
      and "applyCentralModuleMutationSnapshot" in main
      and "foregroundReason += foregroundModuleDeltaMarker" in main,
      "Canonical module PATCHes no longer use persistent registry delta write-through")
check("readModelBatchShouldDefer" in models
      and "readModelMutationQuiet" in models
      and "readModelMutationMaxDeferral" in models
      and "gatewayReadiness.Load() && readModelBatchShouldDefer(events, time.Now().UTC())" in models,
      "Durable reconciliation can still race foreground mutations instead of waiting for the shared quiet window")
check("MaxConnsPerHost:     2" in main
      and "readModelRefreshConcurrency    = 2" in models
      and "centralPartnerWorkspaceSourceConcurrency  = 2" in workspace
      and "centralPartnerWorkspaceGlobalWriteWorkers = 1" in workspace,
      "Background projection/downstream concurrency can still exhaust the four-connection service DB budget")
check("writeThroughCentralPartnerWorkspace(partnerID)" not in write_through
      and "readModelTenantSliceScopes" in write_through
      and "refreshGlobalTenantReadModelSlice" in write_through,
      "Foreground partner mutations can still rebuild the full multi-service tenant workspace before ACK")
check("Configure bounded smoke HTTP clients" in workflow
      and "CURL_HOME=" in workflow
      and "max-time = 90" in workflow,
      "Compose smoke HTTP calls are no longer protected by a global timeout")

cross_tenant_smokes = {
    path for path, source in script_sources.items()
    if "/partner/api/v1/modules" in source
    and (
        "/api/v1/modules/" in source
        or "/api/v1/billing/plans/" in source
        or "/api/v1/cms/design/" in source
    )
}
for required in [
    "scripts/smoke_start_23_11_1.sh",
    "scripts/smoke_start_23_11_3.sh",
]:
    check(required in cross_tenant_smokes,
          f"Cross-tenant mutation/read smoke coverage disappeared: {required}")

check("a.refreshHealthSourceWriteThrough()" in models,
      "System writes no longer synchronously refresh persistent Health compatibility state")
check("if scope.system {" in write_through
      and "context.WithTimeout(context.Background(), 3*time.Second)" in models,
      "Health compatibility write-through is unbounded or not isolated to System mutations")
check("func classifyReadModelMutation" in models
      and 'strings.Contains(path, "/portal-users")' in models
      and 'strings.HasPrefix(path, "/partner/api/v1/")' in models
      and 'partnerMutation := strings.Contains(reason, "partner")' not in models
      and 'len(jobsByKey) == 0 && partnerID == "" && !scope.tenantOnly' in write_through,
      "Partner/tenant mutation classifier can still expand a local mutation into an all-projection refresh storm")
check("type readModelRefreshEvent struct" in models,
      "Durable read-model refresh event type disappeared during write-through refactor")
check("if scope.system {" in write_through
      and 'strings.Contains(foregroundReason, "environment") || strings.Contains(foregroundReason, "provision")' in write_through
      and "add(centralStep4WebsiteKey, a.refreshCentralStep4Website)" in write_through,
      "Provisioning write-through no longer refreshes environment-bearing browser projections")
check('refreshReason += "/audit"' in main,
      "Mutation write-through no longer includes the newly committed audit event")
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
