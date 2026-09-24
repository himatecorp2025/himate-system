#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

common = (ROOT / "services/internal/common/common.go").read_text()
gateway = (ROOT / "services/cmd/gateway/main.go").read_text()
durability = (ROOT / "services/cmd/gateway/phase2_durability.go").read_text()
partner_portal = (ROOT / "services/cmd/gateway/partner_portal.go").read_text()
provisioning = (ROOT / "services/cmd/provisioning/main.go").read_text()
runtime = (ROOT / "services/cmd/runtime/main.go").read_text()
frontend = (ROOT / "frontend/lib/main.dart").read_text()
portal_frontend = (ROOT / "frontend/lib/partner_portal.dart").read_text()
openapi = (ROOT / "docs/openapi.yaml").read_text()
acceptance = (ROOT / "docs/START-23.12_PHASE2_ACCEPTANCE.md").read_text()
ci = (ROOT / ".github/workflows/ci.yml").read_text()

errors = []

def require(condition, message):
    if not condition:
        errors.append(message)

# Migration integrity / rollback compatibility.
require("checksum TEXT NOT NULL DEFAULT ''" in common, "migration registry checksum column is missing")
require("migrationChecksum" in common and "checksum drift detected" in common,
        "migration source drift detection is missing")
require("validateMigrationSafety" in common and "expand-only" in common,
        "expand-only migration safety gate is missing")
require("DROP TABLE " in common and "DROP COLUMN " in common and "TRUNCATE " in common and "DELETE FROM " in common,
        "destructive schema/data patterns are not guarded")

# Durable audit.
require("CREATE TABLE IF NOT EXISTS identity.audit_outbox" in durability, "audit outbox table is missing")
require("createAuditIntent" in durability and "finalizeAuditIntent" in durability,
        "audit outbox lifecycle helpers are missing")
require("recoverAuditOutbox" in durability and "_INTERRUPTED" in durability,
        "restart recovery for audit outbox is missing")
require("AUDIT_DURABILITY" in gateway and "createAuditIntent" in gateway,
        "control-plane mutations do not fail closed on missing audit durability")
require("AUDIT_DURABILITY" in partner_portal and "createAuditIntent" in partner_portal,
        "Partner Portal mutations do not fail closed on missing audit durability")

# Durable onboarding.
require("CREATE TABLE IF NOT EXISTS identity.partner_onboarding_sagas" in durability,
        "Partner onboarding saga table is missing")
require("runPartnerOnboardingSaga" in durability and "onboarding_request_id" in durability,
        "Partner onboarding saga execution/idempotency is missing")
require("onboardingTermsMatch" in durability and "billing terms readback" in durability,
        "Partner onboarding Billing retry reconciliation is missing")
require("portal_owner_password_hash" in durability and "PasswordHash" in durability,
        "Partner onboarding does not persist only a password hash")
require("Only the HIMATE system owner can run partner onboarding" in durability,
        "Partner onboarding lost the System Owner boundary")
require("/api/v1/partner-onboarding" in frontend and "himate_pending_partner_onboarding" in frontend,
        "frontend is not using/resuming durable onboarding")
require("/api/v1/partner-onboarding/{requestId}/resume:" in openapi,
        "durable onboarding resume route is missing from OpenAPI")

# Provisioning.
require("BeginTx(r.Context(),&sql.TxOptions{})" in provisioning,
        "provisioning job/step creation is not transactional")
require('initialStatus:="QUEUED"' in provisioning,
        "non-prepare provisioning jobs are not durably queued before execution")
require("ensureJobSteps" in provisioning,
        "provisioning does not defensively reconcile missing step rows")
require("recoverInterruptedJobs" in provisioning and "status IN ('QUEUED','RUNNING')" in provisioning,
        "provisioning restart recovery worker is missing")

# Deployment intent.
require("CREATE TABLE IF NOT EXISTS runtime.deployment_intents" in runtime,
        "runtime deployment intents table is missing")
require("deploymentRequestKey" in runtime and "ON CONFLICT(request_key) DO NOTHING" in runtime,
        "deployment request idempotency is missing")
require("DEPLOYMENT_RECONCILIATION_REQUIRED" in runtime or "RECONCILIATION_REQUIRED" in runtime,
        "uncertain provider results do not fail closed against duplicate deployment")

# Reload/deep links.
require(portal_frontend.count("Widget usersPage()") == 1 and portal_frontend.count("Widget pageFor(_PortalNavSpec item)") == 1,
        "Partner Portal source contains duplicated navigation/page blocks")
require("portalNavSlug" in portal_frontend and "history.replaceState" in portal_frontend,
        "Partner Portal tab state is not deep-link persisted")
require("startsWith('/partner/app/')" in portal_frontend,
        "Partner Portal does not reconstruct tab state after reload")

# Scope correction: no fake module runtime created to game Phase 2.
require("MODULE_ACCEPTANCE_DEFERRED" in acceptance,
        "module-specific Piano Intake / Workshop Workflow acceptance boundary is not documented")
require("Client Piano Intake" in acceptance and "Workshop Workflow" in acceptance,
        "deferred module-specific crash contracts are not named in the acceptance record")

# CI gating.
require("audit_start_23_12_phase2.py" in ci, "full CI does not run Phase 2 source audit")
require("smoke_start_23_12_phase2.sh" in ci, "full CI does not run Phase 2 onboarding smoke")
require("smoke_start_23_12_phase2_recovery.sh" in ci, "full CI does not run Phase 2 restart recovery smoke")
require("smoke_start_23_12_phase2_runtime.sh" in ci, "full CI does not run Phase 2 deployment intent smoke")

if errors:
    raise SystemExit("START-23.12 Phase 2 source audit failed:\n- " + "\n- ".join(errors))

print("START-23.12 Phase 2 source audit passed: durable onboarding, audit outbox, provisioning recovery, deployment intents, deep links and migration integrity are present.")
