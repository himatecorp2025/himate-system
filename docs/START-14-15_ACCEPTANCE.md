# START-14–15 Acceptance

This document is the release gate for the HIMATE START-14 Evidence and START-15 PDF Reports block.

The scope is derived from the START plan: important results may be linked to Evidence by partner, metric, period and report; Evidence Library cards expose partner/type/metric/period/verification; PDF report types are Partner Impact, Multi-Partner and HIMATE Global Impact reports; PDFs include branding, report identity, partner scope, period, metrics/charts, data sources, Evidence references, document list and generation metadata.

## START-14 — Evidence

- [x] Evidence is implemented as a dedicated containerized Go microservice.
- [x] Evidence types support PDF, image, invoice, contract, screenshot, report, URL, partner declaration and other.
- [x] Every Evidence record requires a partner ID.
- [x] Evidence may be linked to a metric key.
- [x] Evidence retains optional period start/end and rejects reversed periods.
- [x] File-backed Evidence is limited to 20 MiB at the Evidence boundary.
- [x] File-backed Evidence is content-sniffed; declared extension/MIME alone is not trusted.
- [x] PDF/report, image/screenshot and invoice/contract file-type policies are explicit.
- [x] SVG/HTML are not accepted as image/PDF evidence.
- [x] Original filenames are sanitized and never used as filesystem paths.
- [x] Binary objects are stored through the central Storage service, not in Evidence container-local persistent state.
- [x] Storage object keys are namespace-confined and path traversal is rejected.
- [x] Stored file bytes have SHA-256 integrity metadata and stored checksum is cross-checked.
- [x] URL Evidence accepts only absolute HTTP(S) references and never fetches the remote URL.
- [x] Partner declarations are stored as dedicated bounded text records.
- [x] Evidence verification state is explicit: UNVERIFIED, VERIFIED or REJECTED.
- [x] VERIFIED_DOCUMENT Impact provenance requires an Evidence ID.
- [x] VERIFIED_DOCUMENT Impact provenance rejects missing, unverified or cross-partner Evidence.
- [x] VERIFIED_DOCUMENT Impact provenance rejects Evidence linked to another metric.
- [x] VERIFIED_DOCUMENT provenance requires file-backed verified Evidence rather than a URL/declaration-only record.
- [x] Evidence Library supports partner, metric, type, verification and report filters.
- [x] Evidence Library supports period filters, server-side free-text search, limit/offset pagination and total/has-more metadata.
- [x] Evidence detail exposes metric/report relationships and uploader/timestamp metadata.
- [x] File-backed Evidence supports authenticated inline preview.
- [x] File integrity can be actively recomputed against recorded SHA-256 and size.
- [x] Missing stored evidence objects return an explicit broken-reference response rather than failing silently.
- [x] File-backed Evidence can be downloaded through the authenticated Gateway.
- [x] Generated reports can link included Evidence records back to the Evidence Library.
- [x] Responsive Impact & Reports UI includes Evidence upload, verification, download/open and verified-metric workflow.

## START-15 — PDF Reports

- [x] Reports are implemented as a dedicated containerized Go microservice.
- [x] Supported report types are PARTNER_IMPACT, MULTI_PARTNER and HIMATE_GLOBAL.
- [x] PARTNER_IMPACT requires exactly one partner.
- [x] MULTI_PARTNER requires at least two partners.
- [x] HIMATE_GLOBAL is generated without a manually supplied partner scope.
- [x] Every report persists report ID, title, report type, partner scope and period.
- [x] Every report persists a template version.
- [x] Every frozen snapshot has a SHA-256 reference persisted with the report.
- [x] Report metric aggregation is authoritative to the requested report period rather than the all-time live summary.
- [x] Report generation runs as a persisted background job.
- [x] Interrupted RUNNING jobs are recoverable after service restart.
- [x] Metric summaries are frozen into a persistent JSON snapshot before PDF rendering.
- [x] Data-source observations/provenance are frozen into the same snapshot.
- [x] Period-relevant Evidence metadata is frozen into the same snapshot.
- [x] Report PDF rendering reads the frozen snapshot rather than live metric state.
- [x] Regeneration uses the stored snapshot and produces the same PDF checksum for the same snapshot.
- [x] Generated PDFs contain HIMATE branding, title, report ID, partner/global scope, period, metrics and chart bars.
- [x] Generated PDFs print template version, generation timestamp and the frozen snapshot SHA-256 reference.
- [x] Generated PDFs contain data-source references and Evidence/document references.
- [x] Generated PDFs include the START reporting disclaimer that reporting is not an automatic legal determination.
- [x] Generated PDFs are persisted through the central Storage service with SHA-256 and size metadata.
- [x] Reports expose QUEUED/RUNNING/READY/FAILED state and error metadata.
- [x] Completed PDFs are downloadable through the authenticated Gateway.
- [x] Responsive Impact & Reports UI includes report generation, status, evidence count, checksum, download and snapshot regeneration.

## Cross-cutting release gates

Acceptance candidate: `6625579f4acab93590ed1b4d684de23446af5e8c`.

Pre-final evidence: HIMATE CI #291 completed successfully with Render Blueprint validation, Go, Flutter, Docker Compose, START-01–08 regression, START-09–13 integration and START-14–15 integration smoke. Release-gate checkboxes remain open until the same full suite succeeds on the current acceptance candidate or a documentation-only descendant.


- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go build ./cmd/...`
- [ ] `flutter analyze`
- [ ] `flutter test --platform chrome`
- [ ] `flutter build web --release`
- [ ] Render Blueprint validation includes Evidence and Reports services and host bindings.
- [ ] Docker Compose topology validates.
- [ ] All microservice containers build and start.
- [ ] Gateway/private-service health includes Evidence and Reports.
- [ ] START-01–08 regression smoke passes.
- [ ] START-09–13 integration smoke passes.
- [ ] START-14–15 integration smoke passes.
- [ ] The final PR is merged to `develop` only after every release gate is green.
- [ ] A post-merge `develop` run repeats the complete release gate successfully.

START-16 and later work is explicitly out of scope for this block.
