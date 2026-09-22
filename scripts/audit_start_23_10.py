#!/usr/bin/env python3
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        raise SystemExit(f"START-23.10 audit failed: missing {path}")
    return p.read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.10 audit failed: " + message)

runtime = read("services/cmd/runtime/main.go")
runtime_test = read("services/cmd/runtime/main_test.go")
backups_main = read("services/cmd/backups/main.go")
backups_api = read("services/cmd/backups/api.go")
backups_worker = read("services/cmd/backups/worker.go")
connector = read("services/cmd/connector/main.go")
provisioning = read("services/cmd/provisioning/main.go")
environments = read("services/cmd/environments/main.go")
gateway = read("services/cmd/gateway/main.go")
localization = read("frontend/lib/localization.dart")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))
compose = read("docker-compose.yml")
render = read("render.yaml")
openapi = read("docs/openapi.yaml")
acceptance = read("docs/START-23.10_ACCEPTANCE.md")
ci = read(".github/workflows/ci.yml")
smoke = read("scripts/smoke_start_23_10.sh")

def phase_tuple(raw: str):
    value = raw[3:] if raw.startswith("23.") else raw
    return tuple(int(x) for x in value.split("."))

def release_phase(text: str):
    match = re.search(r"0\\.8\\.\\d+-start-23\\.([0-9]+(?:\\.[0-9]+)*)", text)
    return phase_tuple(match.group(1)) if match else ()

for name, text in (("Compose", compose), ("Render", render), ("OpenAPI", openapi)):
    require(release_phase(text) >= (10,), f"{name} release predates START-23.10")

for token in (
    'defaultProvider: strings.ToLower(strings.TrimSpace(common.Env("HIMATE_RUNTIME_PROVIDER", "local")))',
    "validateEnvironmentProvider",
    "PRODUCTION_PROVIDER_REQUIRED",
    'a.defaultProvider != "local"',
):
    require(token in runtime, f"Runtime production-provider guard missing {token!r}")
for token in (
    "TestValidateEnvironmentProviderProductionRejectsLocalDowngrade",
    "TestValidateEnvironmentProviderLocalProcessAllowsLocalForCI",
    "TestDeploymentProviderUsesRenderDefaultAndServiceID",
):
    require(token in runtime_test, f"Runtime provider unit proof missing {token!r}")

require("HIMATE_RUNTIME_PROVIDER: local" in compose, "Compose must retain deterministic local runtime provider")
for token in (
    "HIMATE_RUNTIME_PROVIDER",
    "value: render",
    "RENDER_API_KEY",
    "HIMATE_RENDER_SERVICE_ID",
):
    require(token in render, f"Render provider config missing {token!r}")
require("sk_live_" not in render and "rnd_" not in render, "provider credential literal appears committed")

for token in (
    "func (a *app) runSchedulerOnce() int",
    'queueBackup(item.id,"scheduler")',
    "last_scheduled_at=NOW()",
    "pg_try_advisory_lock(2310001)",
):
    require(token in backups_worker, f"Backup scheduler closure missing {token!r}")
require("LastScheduledAt" in backups_main, "backup policy does not retain last_scheduled_at readback")
require("backups_one_pending_partner_idx" in backups_main, "backup pending-work uniqueness guard is missing")
for token in (
    'parts[0]=="scheduler"&&parts[1]=="run"',
    "func (a *app) schedulerRun",
    '"last_scheduled_at":last',
):
    require(token in backups_api, f"Backup scheduler API closure missing {token!r}")

for token in (
    "if j.Status==\"COMPLETED\" || j.Status==\"CONFIGURATION_REQUIRED\"{return nil}",
    "prepare_only",
    "CREATE_CONNECTOR_CREDENTIAL",
    "CREATE_STAGING_ENVIRONMENT",
):
    require(token in provisioning, f"Provisioning idempotency/infra contract missing {token!r}")
for token in (
    "token_hash",
    "token_returned_once",
    "issueCredential",
    "subtle.ConstantTimeCompare",
    "website-adapter",
    "commercial-state",
):
    require(token in connector, f"Connector credential/website contract missing {token!r}")
for token in (
    'case "verify-domain":',
    'case "deploy":',
    'case "launch":',
    "a.verifyDomain(w,r,id)",
    "a.deployEnvironment(w,r,id)",
    "a.launchEnvironment(w,r,id)",
):
    require(token in environments, f"Environment action contract missing {token!r}")

require('case path == "/api/v1/backups", strings.HasPrefix(path, "/api/v1/backups/"):' in gateway,
        "Gateway does not permission-scope backup operations")
require('return "backups"' in gateway, "Gateway backup RBAC resource is missing")
for token in (
    "func auditPartnerIDFromState",
    "partnerID = auditPartnerIDFromState(newState)",
    "partnerID = auditPartnerIDFromState(requestState)",
):
    require(token in gateway, f"tenant-scoped operational audit enrichment missing {token!r}")

for token in (
    "No website rewrite",
    "Generate or rotate a partner-scoped credential. The raw secret is displayed exactly once.",
    "Database, media and configuration are captured, encrypted and copied to the configured durable backup storage.",
    "Move the partner to READY TO PROVISION before starting provisioning.",
):
    require(token in localization, f"START-23.10 HU localization is missing {token!r}")

rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") == "23.10"]
require(len(rows) == 11, f"expected 11 START-23.10 contracts, found {len(rows)}")
require(phase_tuple(str(matrix.get("completed_through", "0"))) >= (10,), "functional matrix is not completed through START-23.10")
prod_ids = {"BACKUP-CREATE", "BACKUP-RESTORE-TEST"}
for row in rows:
    require(row.get("localization_state") == "COMPLETE", f"{row.get('id')} localization is incomplete")
    if row.get("id") in prod_ids:
        require(row.get("current_state") == "PROD_PROVEN", f"{row.get('id')} lost existing production proof")
    else:
        require(row.get("current_state") == "MUTATION_PROVEN_PROD_UNVERIFIED",
                f"{row.get('id')} is not mutation-proven")
        require("scripts/smoke_start_23_10.sh" in str(row.get("e2e_proof", "")),
                f"{row.get('id')} lacks START-23.10 smoke proof")

for token in (
    "/api/v1/backups/scheduler/run:",
    "/api/v1/provisioning/jobs:",
    "/api/v1/environments:",
    "/api/v1/connectors/{partnerId}/credential:",
):
    require(token in openapi, f"OpenAPI missing START-23.10 contract {token}")

for token in (
    "audit_start_23_9.py",
    "audit_start_23_10.py",
    "smoke_start_23_9.sh",
    "smoke_start_23_10.sh",
    "docs/START-23.10_ACCEPTANCE.md",
):
    require(token in ci, f"CI missing required 23.10 gate {token!r}")

require("HIMATE START-23.10 System & Operations Production Closure smoke passed" in smoke,
        "START-23.10 smoke completion marker is missing")
require("START-23.11 must not begin automatically." in acceptance,
        "23.11 phase boundary is not explicitly preserved")
require("START-23.12" in acceptance and "live Render/DNS/TLS" in acceptance,
        "live-provider proof is not correctly deferred to START-23.12")

print("START-23.10 static audit passed: 11/11 System & Operations contracts closed")
