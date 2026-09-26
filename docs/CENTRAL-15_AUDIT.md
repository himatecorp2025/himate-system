# CENTRAL-15 — Detailed Audit of CENTRAL-12 / CENTRAL-13 / CENTRAL-14

## Audit objective

CENTRAL-15 is a single audit/closure branch. It does not split CENTRAL-12, CENTRAL-13 or CENTRAL-14 into additional development rounds.

The audit compares the original approved user requirements against:
- merged source code on develop,
- backend/domain ownership,
- Flutter behavior,
- static acceptance scripts,
- containerized runtime acceptance,
- the latest full HIMATE CI result.

Baseline:
- CENTRAL-14 PR #86 merged to develop.
- Merge commit: 724faa9dc05cd56589b95d1e1e95ee92c25bbe17.
- Latest accepted CENTRAL-14 head: 0bafaca832ec1e03666e3b3649c3df1c5caeddde.
- HIMATE CI #2477: Go SUCCESS, Flutter SUCCESS, Compose SUCCESS.
- CENTRAL-14 Administration, Backup and Recovery runtime acceptance: SUCCESS.

Machine-readable matrix: docs/CENTRAL-15_AUDIT_MATRIX.json

## Result

Total audited requirements: **35**

- PASS: **30**
- GAP: **5**
- PARTIAL: **0**

CENTRAL-15 is therefore **NOT CLOSED** yet. The five gaps below are concrete source-level mismatches with the approved requirements.

---

## CENTRAL-12 audit

### PASS — Partners performance and functionality
The default Partners read path is served from a persisted backend materialized hot snapshot. The containerized CENTRAL-12 smoke proves the hot response stays below the 800 ms first-usable-data budget.

Filtered/search/paginated views remain authoritative and permission-aware rather than being served from a stale universal snapshot.

### PASS — Licensing & Finance performance
Finance uses a backend materialized read model. The prior unbounded 400 ms Flutter polling loop was removed and replaced by bounded warm-up behavior. The runtime smoke proves hot Finance reads below 800 ms.

### PASS — No infinite loading
Partners/Finance/Package Analytics use truthful loading/warming/empty states. Finance warm-up retry is bounded and analytics empty state does not start a retry loop.

### PASS — PDF empty-data behavior
Partners, Packages, Licensing & Finance and Impact exports all probe availability before download. If no data exists, Flutter shows the localized modal:
- No exportable data
- Nincs exportálható adat

### PASS — Licensing & Finance operational workflow remains functional
The existing workflow still exposes manual invoice draft creation, approval, send, mark-paid, cancel and invoice PDF. The full historical regression suite passed again in HIMATE CI #2477.

### GAP C12-06 — Package Analytics industry/category split is missing
The approved scope required Package Analytics to verify partner counts **and industry split**.

Current backend aggregation in services/cmd/billing/central8.go provides:
- active partner count by package,
- monthly/annual partner counts,
- module usage,
- Portal active time,
- partner rows.

It does **not** select or aggregate partner category/industry and the Flutter Package Analytics UI has no industry/category section.

Required closure:
1. Source authoritative partner category/industry from Partners.
2. Add backend-only industry aggregation to the Package Analytics read model.
3. Render the already-aggregated result in Flutter.
4. Add runtime acceptance that creates multiple partner categories and verifies exact counts.

---

## CENTRAL-13 audit

### PASS — Design controls
The requested labels exist:
- Primary Color
- Brand Color
- Page Background
- Body Text Color

Every color field includes a visible swatch/picker and synchronized HEX value.

Heading/body font catalogs are expanded independently and backend validation accepts the same font families.

### PASS — Layout family is presentation-only
Layout selection is stored in the design model and applied by the website renderer as CSS/layout presentation. CMS page content and mechanical state remain separate.

### PASS — Real device viewport previews exist on the backend
CENTRAL-13 runtime acceptance proves:
- Desktop: 1440 x 900
- Tablet: 834 x 1194
- Mobile: 390 x 844

The raw preview also proves the selected layout, fonts and colors are actually rendered.

### PASS — Publish Active Brand is real
Publish persists the draft as the published design and the public site reads that published state. Runtime acceptance verifies the public HTML after publish.

### PASS — Partner Data Connections
The top-level Klaviyo-specific structure was replaced by partner-first Partner Data Connections.

Partner cards are responsive and show real runtime state:
- ACTIVE
- INACTIVE
- SUSPENDED
- DELETED

The read model comes from Partners + Connector runtime + Website Adapters. Vendor names appear only when a real integration exists.

### GAP C13-07 — automatic real preview is not embedded in the Design Guide
The original request was not only to have preview endpoints/buttons, but to see the **actual automatically loaded preview** in the editor.

Current behavior:
- the inline Design Guide preview is a locally composed illustrative card;
- it uses selected colors/typography to simulate the visual direction;
- clicking it or Desktop/Tablet/Mobile buttons calls createPreview();
- createPreview() opens the real website preview in a new browser tab.

Therefore the real preview exists and works, but it is **not automatically embedded/rendered inside the Design Guide workspace**.

Required closure:
1. Replace/supplement the schematic preview with a real embedded preview frame.
2. Load the backend preview automatically after draft changes with debounce.
3. Provide Desktop/Tablet/Mobile frame switching in-place.
4. Preserve the existing external-open action as a secondary action.
5. Add UI/runtime acceptance proving the embedded frame URL and viewport update after a design mutation.

---

## CENTRAL-14 audit

### PASS — two Administration Centers
The Administration landing contains the two requested large centers:
1. HIMATE Administration Center
2. Partner Administration Center

### PASS — HIMATE Administration Center
The company center contains:
- Financial Administration
- Corporate Documents
- Governance, Settings & Access
- Audit search
- administrator/RBAC management
- company settings
- System Backup & Recovery

Corporate documents are server-searchable.

### PASS — Partner Administration Center
Partners are rendered as cards. Opening a partner fixes the tenant scope and exposes:
- Financial Administration
- Documents
- Audit & Logs
- Backup & Recovery

Partner documents and audit search remain tenant-scoped.

### PASS — verified partner production restore
The production restore path is materially complete and was proven in containerized acceptance:
- READY restore point required;
- PASSED restore test required;
- partner suspension gate;
- exact confirmation;
- reason required;
- automatic safety backup;
- isolated partner DB restore;
- atomic media restore;
- config restore;
- captured runtime release recovery;
- runtime remains suspended for operator validation;
- automatic safety rollback after partial failure;
- backups.approve permission;
- central audit event.

### PASS — platform backup and restore-test
The reserved _platform scope has a default enabled 24-hour backup policy and real scratch-database restore verification.

### GAP C14-06 — new partners are not automatically enrolled into periodic backup
The scheduler iterates only rows already present in backups.policies.

A partner policy is created lazily by ensurePolicy() when:
- a backup is manually queued, or
- the policy endpoint is used.

There is no startup reconciliation or partner-create hook that creates a policy for every active partner. A newly created partner can therefore exist without an automatic scheduled backup.

Required closure:
1. Reconcile the authoritative partner registry into backups.policies.
2. Auto-create the default policy for every eligible active partner.
3. Do this idempotently at startup and/or scheduler cycle.
4. Exclude archived/deleted tenants according to lifecycle policy.
5. Add runtime acceptance: create new partner -> do not touch Backups UI -> scheduler queues its first restore point.

### GAP C14-08 — historical restore point selection is missing from Flutter
The backend already supports:
GET /api/v1/backups?partner_id=<id>

This returns retained restore-point history.

However BackupsPanel production restore always uses:
latest_restore_point_id

There is no operator selector for an earlier verified restore point.

Required closure:
1. Load retained restore-point history for the selected partner.
2. Show created/completed time, status, expiry and restore-test status.
3. Allow only READY + PASSED points to be selected for production restore.
4. Keep latest as the default selection.
5. Add runtime/UI acceptance restoring an older point while a newer point exists.

### GAP C14-09 — HIMATE platform has no executable production maintenance restore procedure
The platform can be backed up and restore-tested, but:
- partnerRestoreAllowed() explicitly rejects _platform production restore;
- the UI labels platform production recovery as maintenance-only;
- no separate platform recovery script or runbook exists in scripts/ or docs/.

That means the system currently proves that a platform backup is restorable in scratch, but does not provide the requested operational path to actually restore the HIMATE control-plane system to a retained point.

Required closure:
1. Keep live self-restore forbidden from the running control plane.
2. Add an offline/maintenance-only platform recovery command or script.
3. Require explicit restore-point ID, confirmation, operator identity and reason.
4. Stop/quiesce dependent services before database replacement.
5. Verify checksum/decrypt/restore in a temporary DB before swap.
6. Perform atomic platform DB replacement or provider-supported restore operation.
7. Re-run migrations only when explicitly compatible; never silently upgrade a restored snapshot.
8. Restart services and run health/acceptance probes.
9. Record a durable recovery audit record/runbook evidence.
10. Add a safe containerized destructive acceptance using an isolated CI database.

---

## Global architecture audit

PASS:
- backend-first read models remain in Go;
- Flutter remains presentation-oriented;
- no unnecessary Administration or Website microservice was introduced;
- existing domain services remain authoritative;
- changes remain containerized;
- the latest full Go/Flutter/Compose regression is green;
- CENTRAL-12, CENTRAL-13 and CENTRAL-14 acceptance gates are all present in the CI chain.

## CENTRAL-15 closure condition

CENTRAL-15 may be marked complete only when all five current GAP items become PASS:

1. C12-06 Package Analytics industry/category split.
2. C13-07 Embedded automatic real Design preview.
3. C14-06 Automatic backup-policy enrollment for every eligible partner.
4. C14-08 Historical restore-point selection.
5. C14-09 Executable maintenance-only HIMATE platform restore procedure.

After those corrections:
- run scripts/audit_central_15.py;
- add a CENTRAL-15 container runtime smoke covering the five closure points;
- run the complete HIMATE CI;
- require Go, Flutter and Compose success before merge.
