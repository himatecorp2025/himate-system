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

partner = read("services/cmd/partners/main.go")
archive = read("services/cmd/partners/compliance_archive.go")
gateway = read("services/cmd/gateway/main.go")
main_dart = read("frontend/lib/main.dart")
archive_dart = read("frontend/lib/compliance_archives.dart")
worker = read("frontend/web/service-worker.js")
storage = read("services/cmd/storage/main.go")
pwa = read("frontend/web/pwa.js")
site = read("frontend/web/site.js")
manifest = read("frontend/web/manifest.json")
index = read("frontend/web/index.html")
phase4 = read("scripts/audit_start_23_12_phase4.py")
ci = read(".github/workflows/ci.yml")
fast = read(".github/workflows/ci-fast.yml")

# Phase 4 is an inherited mandatory baseline, not an optional predecessor.
require("START-23.12 Phase 4" in phase4, "Phase 4 acceptance gate is missing")
require("audit_start_23_12_phase4.py" in ci and "smoke_start_23_12_phase4.sh" in ci,
        "full CI no longer enforces Phase 4 before Phase 5")
require("audit_start_23_12_phase4.py" in fast and "smoke_start_23_12_phase4.sh" in fast,
        "fast CI no longer enforces Phase 4 before Phase 5")

# Seven-year immutable compliance vault.
for token in [
    "CREATE SCHEMA IF NOT EXISTS compliance",
    "CREATE TABLE IF NOT EXISTS compliance.partner_archives",
    "INTERVAL '7 years'",
    "reject_archive_mutation",
    "BEFORE UPDATE OR DELETE",
    "payload_sha256",
    "canonical_payload",
]:
    require(token in archive, f"Compliance Vault invariant missing: {token}")
require("complianceArchiveMigration()" in partner, "Compliance Vault migration is not wired into Partners")
require('p.Lifecycle == "ARCHIVED"' in partner and "archiveComplianceTx" in partner,
        "lifecycle ARCHIVED transition does not atomically create a compliance snapshot")
require("Operational purge was blocked because the seven-year Compliance Archive could not be secured" in partner,
        "operational purge does not fail closed when compliance preservation fails")
require("backfillComplianceArchives" in archive and "a.backfillComplianceArchives(ctx)" in partner,
        "previously archived partners are not backfilled into the seven-year Compliance Vault")
for forbidden in [
    'Schema: "identity", Table: "users"',
    'Schema: "identity", Table: "partner_users"',
    'Schema: "identity", Table: "sessions"',
    'Schema: "identity", Table: "mfa_challenges"',
    'Schema: "identity", Table: "platform_secrets"',
    'Schema: "payments", Table: "partner_profiles"',
]:
    require(forbidden not in archive, f"credential-bearing source leaked into Compliance Vault: {forbidden}")

# Phase 4 service identity and Gateway authorization remain authoritative.
require('path == "/api/v1/archives"' in gateway and 'return "audit"' in gateway,
        "public archive API is not protected by audit.read permission")
require("serveComplianceArchives" in gateway and '"/internal/v1/archives"' in gateway,
        "archive reads do not cross the signed internal Phase 4 boundary")
require("common.DoInternal(a.client, req)" in gateway,
        "archive internal calls bypass the Phase 4 service-signing transport")
require('mux.HandleFunc("/internal/v1/archives"' in partner,
        "Partners service exposes no signed internal archive endpoint")
require('mux.HandleFunc("/api/v1/archives"' not in partner,
        "Compliance Vault is directly exposed on a service /api route")
require("complianceRetentionLocked" in storage and "retain_until>NOW()" in storage,
        "partner file bytes are not locked for the Compliance Archive retention window")
require("r.Method == http.MethodPut || r.Method == http.MethodDelete" in storage and "COMPLIANCE_RETENTION" in storage,
        "archived partner storage can still be overwritten or deleted during compliance retention")

# Central UI exists and is permission-gated.
require("static const int navCount = 10;" in main_dart, "Archives navigation slot is missing")
require("ComplianceArchivesPage" in main_dart and "can('audit.read')" in main_dart,
        "Archives UI is not bound to audit.read")
require("/api/v1/archives" in archive_dart and "SHA-256" in archive_dart,
        "Archives UI does not expose archive integrity/retention evidence")

# PWA/cache is deliberately public-only. Protected app/API content must never be cached.
for prefix in ["/api/", "/partner/", "/app", "/login"]:
    require(prefix in worker, f"service worker protected prefix missing: {prefix}")
require("isProtected(url.pathname)" in worker, "service worker does not short-circuit protected paths")
require("no-store" in worker and "private" in worker, "service worker ignores response cache policy")
require("caches.delete" in worker, "service worker does not retire obsolete caches")
require("navigator.serviceWorker.register('/service-worker.js'" in pwa, "authenticated shell PWA registration is missing")
require("navigator.serviceWorker.register('/service-worker.js'" in site, "public PWA registration is missing")
require('<link rel="manifest" href="/manifest.json">' in index, "Flutter shell manifest link missing")
require('test ! -s build/web/flutter_service_worker.js' in ci and 'Unexpected competing Flutter service worker registration' in ci,
        "release CI does not prevent a second Flutter service worker from overriding the Phase 5 cache policy")
require('"display": "standalone"' in manifest and '"scope": "/"' in manifest, "PWA manifest is incomplete")

# Phase 5 gates must be in both CI paths.
for workflow, name in [(ci, "full CI"), (fast, "fast CI")]:
    require("audit_start_23_12_phase5.py" in workflow, f"{name} does not run Phase 5 architecture audit")
    require("smoke_start_23_12_phase5.sh" in workflow, f"{name} does not run Phase 5 runtime/load acceptance")

print("START-23.12 Phase 5 architecture/compliance/PWA audit: PASS")
