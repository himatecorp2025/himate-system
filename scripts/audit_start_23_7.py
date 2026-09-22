#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        raise SystemExit(f"START-23.7 audit failed: missing {path}")
    return p.read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.7 audit failed: " + message)

billing = read("services/cmd/billing/main.go")
evidence = read("services/cmd/evidence/main.go")
impact = read("services/cmd/impact/main.go")
reports = read("services/cmd/reports/main.go")
catalog = read("services/cmd/catalog/main.go")
frontend = read("frontend/lib/main.dart")
compose = read("docker-compose.yml")
render = read("render.yaml")
ci = read(".github/workflows/ci.yml")
openapi = read("docs/openapi.yaml")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))
helper = read("scripts/evidence_test_helpers.sh")
smoke223 = read("scripts/smoke_start_22_3.sh")
smoke234 = read("scripts/smoke_start_23_4.sh")

# Billing commercial-document boundary.
for token in (
    "EVIDENCE_HOSTPORT",
    "validateCommercialEvidenceReference",
    'strings.HasPrefix(ref, "evidence://")',
    "/integrity",
    "Evidence record does not belong to this partner",
    "commercial Evidence must be backed by a persisted file",
    "Evidence file failed SHA-256 integrity verification",
    "EVIDENCE_REFERENCE_INVALID",
):
    require(token in billing, f"Billing commercial Evidence guard missing {token!r}")

for token in ("EVIDENCE_HOSTPORT: evidence:10000",):
    require(token in compose, f"Compose Billing Evidence binding missing {token!r}")
require("name: himate-billing" in render and "key: EVIDENCE_HOSTPORT" in render and "name: himate-evidence" in render,
        "Render Billing Evidence binding missing")

require("0.8.11-start-23.7" in compose, "Compose release version is not START-23.7")
require("0.8.11-start-23.7" in render, "Render release version is not START-23.7")
require("version: 0.8.11-start-23.7" in openapi, "OpenAPI release version is not START-23.7")
for path in (
    "/api/v1/billing/partners/{partnerId}/documents:",
    "/api/v1/evidence:",
    "/api/v1/evidence/{evidenceId}/integrity:",
    "/api/v1/reports:",
    "/api/v1/reports/{reportId}/regenerate:",
    "/api/v1/modules/{moduleKey}/impact-metrics:",
):
    require(path in openapi, f"OpenAPI missing {path}")
require("same-partner file-backed HIMATE Evidence" in openapi,
        "OpenAPI does not document the START-23.7 commercial Evidence boundary")

# Historical acceptance can no longer inject fabricated commercial evidence.
require(". scripts/evidence_test_helpers.sh" in smoke223, "START-22.3 does not use real Evidence helper")
require(". scripts/evidence_test_helpers.sh" in smoke234, "START-23.4 does not use real Evidence helper")
for stale in (
    "evidence://start223/activation-invoice-001",
    "evidence://start223/payment-001",
    "evidence://start234/activation-invoice",
    "evidence://start234/payment",
):
    require(stale not in smoke223 and stale not in smoke234,
            f"synthetic historical Evidence fixture remains: {stale}")
require("create_pdf_evidence" in helper and "/api/v1/evidence" in helper and "/integrity" in helper,
        "real file-backed Evidence fixture helper incomplete")

# Evidence binary pipeline.
for token in (
    "ParseMultipartForm",
    "readMultipartFile",
    "sniffMime",
    "sha256.New()",
    "putObject",
    'parts[1]=="download"||parts[1]=="preview"',
    'parts[1]=="integrity"',
    "INTEGRITY_MISMATCH",
    "verification_status",
    "VERIFIED",
):
    require(token in evidence, f"Evidence pipeline missing {token!r}")

# Impact definition/value/baseline and verified-document linkage.
for token in (
    "func (a *app) definitions",
    "func (a *app) values",
    "func (a *app) baselines",
    'in.Provenance=="VERIFIED_DOCUMENT"',
    "validateEvidence",
    "EVIDENCE_REQUIRED",
    "EVIDENCE_BOUNDARY",
    "delta_from_baseline",
):
    require(token in impact, f"Impact contract missing {token!r}")

# Module mapping.
require("func (a *app) moduleImpactMetrics" in catalog, "module impact-metric mapping handler missing")
require("catalog.module_impact_metrics" in catalog, "module impact-metric persistence missing")

# Reports must remain frozen-snapshot based.
for token in (
    "buildSnapshot",
    "snapshot_sha256",
    "renderFromStoredSnapshot",
    'parts[1]=="regenerate"',
    "PDFSHA256",
    "pdf_sha256",
    "/internal/v1/evidence/report-links",
):
    require(token in reports, f"report snapshot contract missing {token!r}")
require("a.renderFromStoredSnapshot(r.Context(),rec)" in reports,
        "report regeneration is not explicitly based on the stored snapshot")

# Partner Workspace must upload a real file rather than accepting a free-form storage URL.
for token in (
    "Upload commercial document",
    "pickBrowserFile",
    "widget.api.multipart('/api/v1/evidence'",
    "'storage_url': 'evidence://$evidenceId'",
    "Commercial document uploaded and registered.",
):
    require(token in frontend, f"commercial document UI missing {token!r}")
require("Register commercial document metadata with a persistent storage URL" not in frontend,
        "legacy free-form commercial storage URL UI remains")

# Matrix closure.
require(tuple(map(int, str(matrix.get("completed_through", "0")).split("."))) >= (23, 7),
        "functional matrix is not completed through START-23.7")
rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") == "23.7"]
require(len(rows) == 12, f"expected 12 START-23.7 contracts, found {len(rows)}")
for row in rows:
    require(row.get("current_state") == "MUTATION_PROVEN_PROD_UNVERIFIED",
            f"{row.get('id')} is not mutation-proven")
    proof = str(row.get("e2e_proof", ""))
    require("scripts/smoke_start_23_7.sh" in proof,
            f"{row.get('id')} lacks START-23.7 E2E proof")

# Earlier closure gate must remain mandatory.
require("audit_start_23_1_23_6.py" in ci, "START-23.1-23.6 cross-phase gate disappeared")
require("audit_start_23_7.py" in ci, "CI does not execute START-23.7 static audit")
require("smoke_start_23_7.sh" in ci, "CI does not execute START-23.7 Compose smoke")
require("docs/START-23.7_ACCEPTANCE.md" in ci, "CI does not require START-23.7 acceptance document")

print("START-23.7 static audit passed: 12/12 Evidence, Impact & Reporting contracts closed")
