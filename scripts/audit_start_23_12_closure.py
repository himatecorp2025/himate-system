#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

# All independently accepted phases must remain present.
for path in [
    "docs/START-23.12_PHASE1_ACCEPTANCE.md",
    "docs/START-23.12_PHASE2_ACCEPTANCE.md",
    "docs/START-23.12_PHASE3_ACCEPTANCE.md",
    "docs/START-23.12_PHASE3B_TENANT_INVOICING_ACCEPTANCE.md",
    "docs/START-23.12_PHASE4_SECURITY_ACCEPTANCE.md",
    "docs/START-23.12_PHASE5_ACCEPTANCE.md",
]:
    require((ROOT / path).is_file(), f"missing inherited acceptance document: {path}")

for path in [
    "scripts/audit_start_23_12_phase1.py",
    "scripts/audit_start_23_12_phase2.py",
    "scripts/audit_start_23_12_phase3.py",
    "scripts/audit_start_23_12_phase3b.py",
    "scripts/audit_start_23_12_phase4.py",
    "scripts/audit_start_23_12_phase5.py",
    "scripts/smoke_start_23_12_phase1.sh",
    "scripts/smoke_start_23_12_phase2.sh",
    "scripts/smoke_start_23_12_phase3.sh",
    "scripts/smoke_start_23_12_phase3b.sh",
    "scripts/smoke_start_23_12_phase4.sh",
    "scripts/smoke_start_23_12_phase5.sh",
    "scripts/smoke_start_23_12_closure.sh",
    "scripts/start_23_12_closure_publish.py",
    "docs/START-23.12_CROSS_PHASE_CLOSURE.md",
]:
    require((ROOT / path).is_file(), f"missing inherited acceptance gate: {path}")

serviceauth = read("services/internal/serviceauth/contract.go")
common = read("services/internal/common/common.go")
partner = read("services/cmd/gateway/partner_portal.go")
partner_modules = read("services/cmd/gateway/partner_user_modules.go")
tenant_main = read("services/cmd/tenantfinance/main.go")
tenant_automation = read("services/cmd/tenantfinance/automation_consumer.go")
archive = read("services/cmd/partners/compliance_archive.go")
worker = read("frontend/web/service-worker.js")
compose = read("docker-compose.yml")
render = read("render.yaml")
ci = read(".github/workflows/ci.yml")
fast = read(".github/workflows/ci-fast.yml")

# Phase 3 specific automation HMAC and Phase 4 generic service signature must be cumulative.
for marker in [
    "common.BindInternalRequest(req, a.internalToken)",
    "automation.SignRequest(req, body, financeProducer, a.automationKey",
    "common.DoInternal(a.client, req)",
]:
    require(marker in tenant_automation, f"dual-layer automation transport marker missing: {marker}")
bind_pos = tenant_automation.index("common.BindInternalRequest(req, a.internalToken)")
auto_pos = tenant_automation.index("automation.SignRequest(req, body, financeProducer, a.automationKey")
send_pos = tenant_automation.index("common.DoInternal(a.client, req)")
require(bind_pos < auto_pos < send_pos, "tenant-finance does not apply internal token, automation HMAC and common transport in safe order")
require("serviceauth.Sign(req, token, caller" in common, "Phase 4 transport signer is not centralized in DoInternal")
require(all(marker in serviceauth for marker in ['"client-piano": true', '"workshop": true', '"scheduler": true']),
        "Phase 3 canonical producer identities are rejected by the Phase 4 caller registry")

# Phase 1 authorization remains authoritative for human module execution.
require("a.requirePartnerModuleExecution" in partner_modules, "Partner module runtime no longer requires the Phase 1 execution guard")
require('"billing.write"' in partner_modules and 'partnerInvoiceModuleKey' in partner_modules,
        "manual invoice runtime lost the module + billing permission intersection")

# Phase 2 durable audit must wrap mutations before business routing.
api = partner.split("func (a *app) partnerAPI", 1)[1].split("func (a *app) partnerDashboard", 1)[0]
require("createAuditIntent" in api and "finalizeAuditIntent" in api, "Phase 2 durable audit intent/finalization is not wrapped around Partner mutations")
require(api.index("createAuditIntent") < api.index("switch{"), "Partner mutation audit intent is not persisted before business routing")
require("stripUntrustedAuthorityHeaders(r)" in api, "Phase 4 authority stripping is missing from the Phase 2/Phase 1 mutation path")

# Automation is a transport, not an entitlement authority.
require("catalogHost" in tenant_main and "CATALOG_HOSTPORT" in tenant_main,
        "tenant-finance has no Catalog authority dependency")
require("partnerModuleEntitled" in tenant_automation and "invoiceModuleKey" in tenant_automation,
        "automated invoice intake does not re-check organization entitlement")
ent_pos = tenant_automation.index("partnerModuleEntitled(ctx, intent.PartnerID, invoiceModuleKey)")
create_pos = tenant_automation.index("a.createAutomatedReady(", ent_pos)
require(ent_pos < create_pos, "automated invoice is created before invoice_documents entitlement is proven")
require("organization entitlement is not ACTIVE and executable" in tenant_automation,
        "automated finance entitlement failure is not fail-closed")

# Phase 2 audit + Phase 3B finance evidence must survive Phase 5 archival without secret domains.
for marker in [
    '{Key: "administrative_audit"',
    '{Key: "tenant_invoices"',
    '{Key: "tenant_invoice_items"',
    '{Key: "tenant_invoice_events"',
]:
    require(marker in archive, f"Compliance Vault misses cross-phase evidence source: {marker}")
for sensitive in [
    '"identity.users"', '"identity.partner_users"', '"identity.sessions"',
    '"identity.mfa_challenges"', '"identity.platform_secrets"', '"payments.partner_profiles"',
]:
    require(sensitive in archive, f"Compliance Vault sensitive exclusion missing: {sensitive}")

# Phase 5 PWA remains public-only after all backend hardening.
for protected in ["/api/", "/partner", "/app"]:
    require(protected in worker, f"PWA worker does not explicitly protect authenticated surface {protected}")
require("cache.put" in worker and "response.ok" in worker, "public PWA cache contract is missing")

# Local regression defaults stay compatible, but final closure can turn on the exact Render security mode.
require(compose.count('HIMATE_REQUIRE_SERVICE_SIGNATURE: "${HIMATE_REQUIRE_SERVICE_SIGNATURE:-false}"') >= 19,
        "Compose cannot switch the private topology into production service-signature mode")
require("CATALOG_HOSTPORT: catalog:10000" in compose, "tenant-finance Catalog authority is missing from Compose")
require(render.count("key: HIMATE_REQUIRE_SERVICE_SIGNATURE") >= 19 and render.count('value: "true"') >= 19,
        "Render private services do not enforce service signatures")
tf = render.split("name: himate-tenant-finance", 1)[1].split("\n  - type:", 1)[0]
require("key: CATALOG_HOSTPORT" in tf and "name: himate-catalog" in tf,
        "tenant-finance production deployment is not bound to Catalog authority")

# Full CI must prove every phase, Phase 3B, and the final closure in order.
audit_steps = [
    "audit_start_23_12_phase1.py",
    "audit_start_23_12_phase2.py",
    "audit_start_23_12_phase3.py",
    "audit_start_23_12_phase3b.py",
    "audit_start_23_12_phase4.py",
    "audit_start_23_12_phase5.py",
    "audit_start_23_12_closure.py",
]
pos = -1
for step in audit_steps:
    next_pos = ci.find(step)
    require(next_pos > pos, f"full CI audit ordering/presence broken at {step}")
    pos = next_pos

runtime_steps = [
    "smoke_start_23_12_phase1.sh",
    "smoke_start_23_12_phase2.sh",
    "smoke_start_23_12_phase3.sh",
    "smoke_start_23_12_phase3b.sh",
    "smoke_start_23_12_phase4.sh",
    "smoke_start_23_12_phase5.sh",
    "smoke_start_23_12_closure.sh",
]
pos = -1
for step in runtime_steps:
    next_pos = ci.find(step)
    require(next_pos > pos, f"full CI runtime ordering/presence broken at {step}")
    pos = next_pos

require("audit_start_23_12_closure.py" in fast, "Fast CI does not protect the cross-phase closure contract")
require("smoke_start_23_12_closure.sh" not in fast,
        "production-signature topology recreation belongs to full CI, not the fast compatibility lane")

print("HIMATE START-23.12 Phase 1-5 Cross-Phase Production Acceptance Closure audit: PASS")
