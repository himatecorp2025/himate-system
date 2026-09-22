# START-23.8 Acceptance — CMS, Design & SEO Completion

## Purpose

START-23.8 closes the CMS, Design Guide and SEO contracts left open by the START-23.1 functional reset.

The phase does not redesign the public website. It completes the existing versioned CMS architecture so source-independent sections and pages can render in the initial HTML, previews are real website previews rather than raw JSON, published Design settings affect the public HTML, and published SEO settings are provable in the browser response.

## Scope

START-23.8 closes exactly these functional-matrix contracts:

- `CMS-PAGE-CREATE`
- `CMS-DRAFT-SAVE`
- `CMS-MEDIA-UPLOAD`
- `CMS-PREVIEW`
- `CMS-PUBLISH`
- `CMS-ROLLBACK`
- `CMS-ARBITRARY-SECTION`
- `DESIGN-DRAFT`
- `DESIGN-PUBLISH`
- `DESIGN-PREVIEW`
- `SEO-DRAFT`
- `SEO-PUBLISH`

No START-23.9+ product scope is included.

## CMS page contract

### Page and locale model

CMS pages remain identified by stable `page_key` plus locale.

The same page key may have independent `en_US` and `hu_HU` variants.

Each locale owns an independent version chain:

- DRAFT
- PREVIEW
- PUBLISHED

Publishing requires a fresh PREVIEW whose `source_version_id` equals the current draft.

### Draft

`PUT /api/v1/cms/pages/{pageId}/draft` persists the complete version input.

Supported sections remain validated against the CMS component allowlist.

Draft content must never leak into the public route before publish.

### Arbitrary sections

A CMS section no longer has to exist in a source-controlled marketing HTML template.

When a visible published or preview section does not match an existing `data-cms-section` block, Gateway must generate a safe public component that:

1. escapes all textual content;
2. uses the validated CTA URL;
3. uses the correct public or token-bound preview media URL;
4. retains the stable section ID in `data-cms-section`;
5. uses a normalized component CSS class;
6. is inserted into the dynamic CMS root when present, otherwise before the footer;
7. receives responsive dynamic-section CSS.

Existing source-template sections continue to be edited in place.

### Arbitrary pages

A published CMS slug that is not one of the source-controlled marketing routes must be served directly by Gateway instead of redirecting to the landing page.

Gateway builds the page from the generic HIMATE public shell and the published CMS page JSON.

An unpublished arbitrary slug must not render publicly.

## Page preview contract

`POST /api/v1/cms/pages/{pageId}/preview` remains API-compatible and still returns:

- `preview_token`
- raw `preview_path`

START-23.8 additionally returns:

- `preview_html_path`

The full HTML preview is served by:

`GET /cms-preview/{slug}?token=...`

Required properties:

- token-bound;
- private / no-store;
- `X-Robots-Tag: noindex, nofollow`;
- initial HTML, not JSON;
- same CMS SSR renderer used by published pages;
- preview media remains token-bound;
- hidden sections remain absent;
- arbitrary sections are rendered as real components.

The admin CMS Preview action must open `preview_html_path`.

## Publish and rollback contract

### Publish

`POST /api/v1/cms/pages/{pageId}/publish` must make the selected preview visible in the Gateway initial HTML.

Proof must include:

- title/meta/canonical;
- visible section text;
- arbitrary section markup;
- published media path;
- English/Hungarian locale selection.

### Rollback

`POST /api/v1/cms/pages/{pageId}/rollback` creates a new PUBLISHED version from a previous published version.

The public HTML must immediately return the rolled-back content while preserving the immutable history and rollback lineage.

## CMS media contract

CMS media remains Storage-backed and SHA-256 protected.

START-23.8 acceptance must prove the same bytes through:

- preview media URL protected by the page preview token;
- public media URL after publication.

## Design Guide contract

### Draft

`PUT /api/v1/cms/design/draft` remains the authoritative draft mutation.

Draft readback must preserve:

- logo media ID;
- brand colors;
- heading/body font;
- button radius;
- bilingual navigation.

### Preview

`POST /api/v1/cms/design/preview` creates a random preview token.

Only the SHA-256 token hash is persisted in `cms.site_design`.

The token:

- expires after 30 minutes;
- is required for draft design JSON;
- is required for the draft logo media path;
- does not expose the draft through a public endpoint.

Full website preview is served by:

`GET /design-preview?token=...&viewport=desktop|tablet|mobile`

Required viewport wrappers:

- desktop: 1440 px;
- tablet: 834 px;
- mobile: 390 px.

The wrapper loads the actual landing page render in a same-origin iframe. Security headers may allow same-origin framing only on this tokenized preview endpoint; all other pages retain frame denial.

The raw preview render must contain the actual public website HTML with the draft design applied.

### Publish

`POST /api/v1/cms/design/publish` increments the design version.

Gateway must apply a published design server-side to public HTML only after an explicit publish (`version > 0`).

The published design affects:

- CSS brand variables;
- page background and text color;
- heading/body font stack;
- button radius;
- published logo asset;
- header navigation;
- footer navigation;
- locale-specific navigation labels.

## SEO contract

### Draft

`PUT /api/v1/cms/seo/draft` persists independent English and Hungarian global keyword sets, Organization metadata and optional default OG image.

### Publish

`POST /api/v1/cms/seo/publish` increments the SEO version.

Published global SEO must be observable in initial public HTML:

- CMS pages receive combined page + global keywords;
- CMS JSON-LD contains the published Organization as publisher;
- static fallback pages receive global keywords;
- static fallback pages receive Organization JSON-LD.

No JavaScript execution may be required for these tags to exist.

## Security contract

- CMS preview tokens are stored only as hashes.
- Design preview tokens are stored only as hashes.
- Design preview token lifetime is 30 minutes.
- Preview responses are `private, no-store`.
- Preview responses carry `noindex, nofollow`.
- Public design is never read from draft state.
- Only `/design-preview` may relax frame protection to SAMEORIGIN / `frame-ancestors 'self'`; all other routes retain frame denial.
- Dynamic CMS text is HTML-escaped.
- CTA URLs remain subject to existing CMS URL validation.

## Required automated proof

### Static audit

`scripts/audit_start_23_8.py` must verify at least:

- full-page CMS preview contract and frontend wiring;
- arbitrary dynamic section generation;
- arbitrary published slug rendering;
- public design fetch and server-side design application;
- hashed design preview token migration and 30-minute validation;
- desktop/tablet/mobile design preview paths and admin controls;
- published SEO rendering path;
- START-23.8 matrix closure;
- START-23.1–23.6 cross-phase guard remains active;
- START-23.7 audit remains active;
- CI executes START-23.8 static and Compose gates.

### Compose mutation smoke

`scripts/smoke_start_23_8.sh` must prove:

1. checksum-backed CMS media upload;
2. English arbitrary CMS page create;
3. full HTML preview token;
4. noindex/no-store preview behavior;
5. arbitrary section is generated in preview HTML;
6. preview media bytes equal the uploaded bytes;
7. unpublished arbitrary slug is not public;
8. publish produces initial server-rendered HTML;
9. public media bytes equal uploaded bytes;
10. Hungarian sibling page on the same slug;
11. language-specific public HTML;
12. draft save;
13. second publish;
14. rollback restores the earlier public HTML;
15. Design Guide draft readback;
16. Desktop 1440 preview;
17. Tablet 834 preview;
18. Mobile 390 preview;
19. raw design preview applies draft CSS/navigation;
20. Design publish mutates public HTML;
21. published navigation uses English/Hungarian labels correctly;
22. SEO draft readback;
23. SEO publish inserts global keywords in initial HTML;
24. SEO publish inserts Organization JSON-LD;
25. static fallback page receives published global SEO;
26. immutable page version history and CMS audit preserve the workflow;
27. global Design/SEO fixture values are restored after the smoke.

## Release contract

START-23.8 release identifier:

`0.8.12-start-23.8`

## Definition of Done

START-23.8 is complete only when:

- Go tidy/vet/unit/race/OpenAPI/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23 through START-23.7 static audits remain green;
- START-23.1–23.6 cross-phase audit remains green;
- START-23.8 static audit passes;
- complete historical Docker Compose regression remains green;
- START-23.8 mutation smoke passes;
- all 12 START-23.8 functional-matrix contracts are closed;
- pull request is merged into `develop`;
- the merge commit's `develop` push CI is green.
