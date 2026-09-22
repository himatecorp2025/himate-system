# START-23.7 Acceptance — Evidence, Impact & Reproducible Reporting

## Purpose

START-23.7 closes the Evidence / Impact / Reporting contracts left open by the START-23.1 functional reset.

The phase turns previously source-complete controls into end-to-end proven business workflows and closes the remaining commercial-document metadata gap: commercial evidence must be backed by a real HIMATE Evidence/Storage object, not an arbitrary URL string.

## Scope

START-23.7 closes exactly these functional-matrix contracts:

- `PART-WIZ-INVOICE`
- `PART-DOCUMENT-REGISTER`
- `MODULE-METRIC-MAP`
- `IMPACT-DEFINITION`
- `IMPACT-VALUE`
- `IMPACT-BASELINE`
- `EVIDENCE-METADATA`
- `EVIDENCE-UPLOAD`
- `EVIDENCE-VERIFY`
- `EVIDENCE-VERIFIED-VALUE`
- `REPORT-CREATE`
- `REPORT-REGENERATE`

No START-23.8+ product scope is included.

## Commercial document contract

Commercial Billing documents with kind:

- `CONTRACT`
- `INVOICE`
- `RECEIPT`
- `PAYMENT_EVIDENCE`

must use a HIMATE Evidence reference in the form:

`evidence://<evidence-id>`

For every commercial registration, Billing must verify through the Evidence service that:

1. the Evidence record exists;
2. it belongs to the same partner;
3. it is backed by a persisted file;
4. metadata contains a non-empty SHA-256 and positive byte size;
5. a fresh Evidence integrity check can read the stored bytes;
6. the freshly calculated SHA-256 equals the persisted SHA-256;
7. the freshly calculated byte size equals the persisted byte size.

Billing copies the authoritative MIME type, SHA-256 and size from Evidence instead of trusting client-supplied values.

An arbitrary URL or a fabricated `evidence://` identifier must fail closed.

The Partner Workspace must therefore upload the file through `POST /api/v1/evidence` first, then register the returned Evidence identifier with Billing.

## Evidence contract

### Metadata Evidence

`POST /api/v1/evidence` with JSON supports:

- `URL`
- `PARTNER_DECLARATION`

URL Evidence is a reference only and is never server-side fetched.

### File Evidence

Multipart `POST /api/v1/evidence`:

- is partner-scoped;
- requires an allowed Evidence type;
- is bounded to 20 MiB;
- content-sniffs the uploaded bytes rather than trusting the browser MIME type;
- persists the file through the Storage service;
- stores original filename, detected MIME type, byte size and SHA-256;
- verifies that the Storage service returned the same SHA-256.

Supported file-backed workflows include PDF/image/invoice/contract/screenshot/report/other according to the service allowlist.

### File retrieval and integrity

- `GET /api/v1/evidence/{id}/preview` returns the stored file inline.
- `GET /api/v1/evidence/{id}/download` returns the stored file as an attachment.
- `GET /api/v1/evidence/{id}/integrity` rereads the current Storage bytes and recomputes SHA-256 and size.
- missing or mutated stored bytes must fail closed.

### Verification

`PATCH /api/v1/evidence/{id}` changes verification status.

A `VERIFIED_DOCUMENT` Impact observation may only reference Evidence that is:

- `VERIFIED`;
- file-backed;
- owned by the same partner;
- linked to the same metric key.

## Impact contract

### Metric definition

`POST /api/v1/impact/definitions` persists stable metric keys with independent English and Hungarian labels/descriptions.

### Module mapping

`PUT /api/v1/modules/{moduleKey}/impact-metrics` persists the metric-key mapping used by the module control plane.

A subsequent GET must return the persisted mapping.

### Observation

`POST /api/v1/impact/values` supports administrator-authored:

- `MANUAL`
- `VERIFIED_DOCUMENT`

provenance only.

Evidence identifiers are rejected for non-`VERIFIED_DOCUMENT` observations.

### Baseline

`PUT /api/v1/impact/baselines` persists a partner/metric baseline.

Impact summary readback must expose the baseline and calculate the delta from the aggregated current value.

## Report contract

### Create

`POST /api/v1/reports` creates a report job.

The report process must:

1. read the requested partner/metric/evidence state;
2. build a frozen JSON snapshot;
3. calculate and persist the snapshot SHA-256;
4. persist the Evidence identifiers captured in that snapshot;
5. render a PDF from the frozen snapshot;
6. persist the PDF through Storage;
7. persist the PDF SHA-256 and byte size;
8. mark the report `READY`;
9. link captured Evidence records back to the report.

### Download

`GET /api/v1/reports/{id}/download` must return a real PDF whose SHA-256 matches the report metadata.

### Regenerate

`POST /api/v1/reports/{id}/regenerate` must render from the **stored snapshot only**.

It must not reread live Impact/Evidence data. If live metrics change after the original report becomes READY, regeneration must retain the same snapshot SHA-256 and deterministic PDF SHA-256.

## Historical regression upgrade

START-22.3 and START-23.4 previously used synthetic `evidence://...` strings in acceptance fixtures.

START-23.7 upgrades those historical smokes to create real file-backed Evidence objects before Billing document registration. This preserves the historical business acceptance while removing the synthetic evidence bypass.

## Deployment topology

Billing now requires:

`EVIDENCE_HOSTPORT`

in both Docker Compose and Render Blueprint wiring.

The service remains private; Billing calls Evidence with the existing internal service token.

Release contract version: `0.8.11-start-23.7`.

## Required automated proof

### Static audit

`scripts/audit_start_23_7.py` must verify:

- Billing has an Evidence service dependency;
- commercial documents require `evidence://` and fresh integrity validation;
- Partner Workspace uploads files through Evidence instead of accepting a free-form storage URL;
- Evidence upload/preview/download/integrity/verification paths remain implemented;
- VERIFIED_DOCUMENT provenance remains Evidence-bound;
- report creation/regeneration remain snapshot-backed;
- all 12 START-23.7 matrix contracts are closed with concrete proof;
- the START-23.1–23.6 cross-phase guard remains enabled;
- CI executes START-23.7 static and Compose gates.

### Compose mutation smoke

`scripts/smoke_start_23_7.sh` must prove:

1. metric definition creation;
2. module-to-metric mapping write/readback;
3. manual observation write/readback;
4. baseline write and summary delta;
5. URL Evidence;
6. Partner Declaration Evidence;
7. real PDF Evidence upload;
8. preview and download of stored bytes;
9. SHA-256 integrity readback;
10. Evidence verification;
11. VERIFIED_DOCUMENT observation linked to the verified file;
12. fabricated Billing Evidence reference rejected;
13. real Evidence-backed activation invoice registered in Billing;
14. report queues and reaches READY;
15. report snapshot SHA-256 and PDF SHA-256 are persisted;
16. downloaded report is a real PDF and matches the persisted SHA-256;
17. live metric mutation after report creation does not alter regeneration snapshot/PDF;
18. report-to-Evidence linkage is persisted;
19. representative mutation audit remains visible.

## Definition of Done

START-23.7 is complete only when:

- Go tidy/vet/unit/race/OpenAPI/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23 through START-23.6 static audits remain green;
- START-23.1–23.6 cross-phase audit remains green;
- START-23.7 static audit passes;
- complete historical Docker Compose regression remains green;
- upgraded START-22.3 and START-23.4 Evidence fixtures remain green;
- START-23.7 mutation smoke passes;
- all 12 START-23.7 functional-matrix contracts are closed;
- the pull request is merged into `develop`.
