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
smoke0108 = read("scripts/smoke_backend.sh")
smoke0913 = read("scripts/smoke_start_09_13.sh")
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

require("HIMATE_APP_VERSION" in compose and "-start-23." in compose, "Compose release contract is missing")
require("HIMATE_APP_VERSION" in render and "-start-23." in render, "Render release contract is missing")
require("version: 0.8." in openapi and "-start-23." in openapi, "OpenAPI release contract is missing")
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
for label, smoke in (
    ("START-01-08", smoke0108),
    ("START-09-13", smoke0913),
    ("START-22.3", smoke223),
    ("START-23.4", smoke234),
):
    require(". scripts/evidence_test_helpers.sh" in smoke,
            f"{label} does not use real Evidence helper")
for stale in (
    "ci://receipt/paid.pdf",
    "ci://start09/receipt.pdf",
    "ci://start09/isolation.pdf",
    "evidence://start223/activation-invoice-001",
    "evidence://start223/payment-001",
    "evidence://start234/activation-invoice",
    "evidence://start234/payment",
):
    require(all(stale not in smoke for smoke in (smoke0108, smoke0913, smoke223, smoke234)),
            f"synthetic historical Evidence fixture remains: {stale}")
require("create_pdf_evidence" in helper and "/api/v1/evidence" in helper and "/integrity" in helper,
        "real file-backed Evidence fixture helper incomplete")
require('-F "metric_key=$metric_key"' in helper,
        "Evidence fixture helper drops metric linkage required by VERIFIED_DOCUMENT provenance")

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

# START-23.11.3e deliberately removed Evidence/Billing hard dependencies from
# New Partner creation. Commercial documents remain file-backed, but they are
# completed from Partner Workspace after the authoritative partner record exists.
add_partner_start = frontend.index("  Future<void> addPartner() async {")
add_partner_end = frontend.index("\n  List<Map<String, dynamic>> get filtered", add_partner_start)
add_partner = frontend[add_partner_start:add_partner_end]
for stale in (
    "activationInvoiceFile",
    "paymentEvidenceFile",
    "activationInvoiceReference",
    "evidenceReference",
    "widget.api.multipart('/api/v1/evidence'",
    "/documents'",
):
    require(stale not in add_partner,
            f"New Partner creation must not be hard-coupled to commercial Evidence: {stale}")
require(
    "Agreements, invoices, payment evidence, modules and environments can be completed from the partner workspace." in add_partner,
    "New Partner UI does not explain the deferred commercial Evidence workflow",
)
# The authoritative post-create workspace still owns the real Evidence pipeline.
# START-23.11.3e generalized the document uploader, so verify the semantic
# INVOICE -> Evidence INVOICE mapping rather than requiring a hard-coded payload.
add_document_start = frontend.index("  Future<void> addDocument() async {")
add_document_end = frontend.index("\n  Map<String, dynamic>? subscriptionFor", add_document_start)
add_document = frontend[add_document_start:add_document_end]
for token in (
    "final evidenceType = switch (kind)",
    "'INVOICE' => 'INVOICE'",
    "'evidence_type': evidenceType",
    "'storage_url': 'evidence://$evidenceId'",
    "Commercial document uploaded and registered.",
):
    require(token in add_document, f"Partner Workspace Evidence flow missing {token!r}")

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
