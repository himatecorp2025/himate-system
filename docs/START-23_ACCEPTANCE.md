# START-23 Acceptance — Responsive & Functional QA

## Scope

START-23 is a final-readiness quality gate over the completed START-01–22.3 system. It does not add a new business domain. Its purpose is to make the existing administration, Partner Portal and public/control-plane flows behave reliably across supported viewport sizes and common UI states before START-24 Security Acceptance.

START-23 begins only after START-22.3 and the full pre-START-23 branch/develop/backend/frontend/privacy/load audit are green.

## Explicit exclusions

START-23 does **not**:
- replace or reprogram partner websites;
- add direct partner-database access;
- copy Klavierhaus business data into HIMATE;
- introduce the real Klavierhaus bridge beyond the accepted Connector boundary;
- redefine authorization, cryptographic or security-acceptance policy;
- replace START-24 Security Acceptance.

Security defects discovered during QA still fail closed and may be corrected, but START-24 remains the dedicated final security gate.

## Responsive viewport matrix

The administration shell, shared dialogs and responsive primitives are acceptance-tested at:

- 320×568 — small phone;
- 390×844 — standard phone;
- 768×1024 — tablet portrait;
- 1024×768 — tablet / compact laptop;
- 1366×768 — common laptop;
- 1440×900 — desktop.

Small-phone checks include 1.3× text scaling.

### Required layout behavior

- shell navigation uses mobile, tablet and desktop modes;
- short-height desktop login layouts fall back to the compact composition;
- form field pairs stack before horizontal crowding;
- page-header actions wrap/stack before overflow;
- dialog actions remain usable on narrow screens;
- KPI grids adapt 1 → 2 → 4 columns;
- long partner/company/domain/status values are bounded or wrap safely;
- record-oriented surfaces remain cards rather than non-responsive raw data tables;
- section headers, information cards, message cards, rule strips and status pills remain readable at narrow widths;
- touch-target actions remain individually selectable and are not visually clipped.

## Functional route matrix

The authenticated System Owner QA flow must reach the primary read surfaces used by the Flutter application:

- Dashboard;
- Partners and pagination;
- Module Control Plane;
- Billing/company profile;
- Impact;
- Evidence;
- Reports;
- CMS pages, media and SEO;
- Contact Leads;
- System Health;
- Provisioning;
- Environments;
- Backups;
- START-22 Connector summary;
- Administration roles and users;
- Audit;
- Notifications;
- Profile / identity.

The public landing route and Partner Portal login route must also remain reachable.

## State coverage

Automated and existing regression tests together must cover:

- loading states;
- error/unavailable states;
- empty result sets;
- pagination;
- long text/data values;
- create/edit dialogs;
- success/failure feedback;
- owner versus non-owner administration behavior;
- Partner Portal tenant isolation;
- commercial activation/provisioning flow;
- backup/recoverability status;
- notification center availability;
- CMS draft/preview/publish workflows.

## Bilingual consistency

START-23 closes known English/Hungarian gaps on critical control-plane surfaces introduced through START-22.3 and the post-22.3 backup correction, including:

- Administration and custom roles;
- commercial agreement and Partner Website Adapter;
- backup/recoverability wording;
- notifications;
- Partner Portal;
- Module Control Plane;
- CMS dynamic version/audit labels.

Dynamic CMS/commercial status messages must use the same localization boundary as static UI labels.

## Performance / load gate

START-23 keeps the existing post-START-22.3 concurrent load audit as a mandatory regression gate:

- liveness;
- public landing page;
- paginated Partners;
- System Health fan-out.

No START-23 UI change may regress those established limits.

## Automated acceptance

1. Go tidy, vet, unit, race and service build.
2. Flutter analyze.
3. Flutter browser tests, including the START-23 viewport/text-scale matrix.
4. Flutter release web build.
5. Static START-23 frontend audit.
6. Render Blueprint service-binding audit.
7. Docker Compose topology and private-service health.
8. Full START-01–22.3 historical regression.
9. Post-START-22.3 privacy and credential-boundary audit.
10. Post-START-22.3 concurrent load audit.
11. START-23 functional route-matrix smoke.
12. No START-24 feature work is included.

## Definition of Done

START-23 is DONE only when its branch is based on the latest audited `develop`, all acceptance gates are green, no unique work remains outside the merge path, and the final PR is merged into `develop`.

After merge, development stops for a short closure audit. The next development block is START-24 Security Acceptance.
