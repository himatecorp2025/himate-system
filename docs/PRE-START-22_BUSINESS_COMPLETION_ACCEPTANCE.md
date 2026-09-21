# Pre-START-22 Business Completion Acceptance

This release gate closes the four business-completeness gaps identified after the START-01–20 audit. It does not redefine START-21 backups/recovery or begin the next numbered START block.

## 1. Password complexity

- [x] All newly accepted passwords require at least 12 characters.
- [x] A password must contain at least one lowercase letter.
- [x] A password must contain at least one uppercase letter.
- [x] A password must contain at least one number.
- [x] A password must contain at least one special/punctuation character.
- [x] The rule is authoritative in the Gateway backend, not only in Flutter.
- [x] Bootstrap owner password validation uses the same policy.
- [x] Profile password changes verify the current password and use the same policy.
- [x] Administrator creation/password replacement uses the same policy.
- [x] Flutter Profile and Administration surfaces explain and validate the same rule.
- [x] Password/session rotation and secret redaction behavior from the existing identity acceptance remain unchanged.

## 2. Contact Leads

- [x] The public Contact form continues to persist the inquiry through the dedicated Contact microservice.
- [x] Persisted inquiries have an explicit lead workflow state: NEW, IN_PROGRESS, CONTACTED or CLOSED.
- [x] Leads support an internal assignee and bounded internal follow-up note.
- [x] Authenticated lead listing supports server-side search, status filter, limit/offset pagination and total/has-more metadata.
- [x] Individual leads can be read and updated through authenticated admin APIs.
- [x] Website & Marketing exposes a responsive Contact Leads workspace.
- [x] The admin workspace supports search, status filtering, inquiry detail, assignee, internal note and mailto follow-up.
- [x] Contact read/write is controlled by backend RBAC.
- [x] A dedicated Marketing Admin role can manage CMS and Contact Leads without Finance or Administration access.
- [x] Contact-lead mutations generate central Gateway audit events.
- [x] Source IP and user-agent fields remain server-side operational metadata and are not exposed by the admin lead DTO.

## 3. Complete English/Hungarian baseline

- [x] The account preference remains the authoritative admin locale: en_US or hu_HU.
- [x] Login, profile and navigation key-based translations remain available in both languages.
- [x] Legacy hard-coded admin Text widgets now pass through a locale-aware rendering layer.
- [x] Form labels, hints, helper text and tooltips use the same locale-aware layer.
- [x] Core Partner, Finance, Impact, Evidence, Reports, CMS, Operations, Backups, Administration, Contact Leads and Design Guide terminology has Hungarian coverage.
- [x] Public marketing pages resolve language from explicit ?lang, persisted preference/cookie, then browser language.
- [x] Public pages provide an EN/HU language switch without changing the approved page layout.
- [x] The source-controlled public English content has Hungarian translation coverage.
- [x] Public Contact workflow status messages are localized.
- [x] CMS page identity is locale-aware; the same stable page key may have independent en_US and hu_HU variants.
- [x] Published public CMS lookup and manifest are locale-scoped.
- [x] Gateway SSR requests the selected CMS locale and sets the HTML/content language for Hungarian CMS content.
- [x] Locale-aware date/currency formatting remains centralized.

## 4. Global Website Design Guide

- [x] Website & Marketing exposes a dedicated responsive Design Guide editor.
- [x] The Design Guide controls the public logo using a validated CMS media asset.
- [x] It controls primary navy, brand gold, page background and body-text colors using validated six-digit hexadecimal values.
- [x] It controls heading and body typography from an allowlisted font set.
- [x] It controls the primary button corner radius.
- [x] It manages ordered public navigation items with independent English/Hungarian labels, URL and visibility.
- [x] Navigation URLs are restricted to safe internal paths or absolute HTTP(S) URLs.
- [x] Design settings use explicit DRAFT and PUBLISHED states.
- [x] Saving and publishing Design Guide state is audited.
- [x] Public pages consume only the PUBLISHED design state.
- [x] If Design Guide delivery is unavailable, the existing approved source-controlled design remains the fallback.
- [x] A published Design Guide logo is allowed through the public CMS media boundary.
- [x] Existing per-page CMS heading/body/media/CTA/visibility/section-order/version/preview/publish/rollback behavior remains intact.

## Regression and release gates

This correction is accepted only when the branch CI proves all of the following on the same candidate commit:

- [ ] Go vet.
- [ ] Go unit tests.
- [ ] Go race tests.
- [ ] Go build for all commands.
- [ ] Flutter analyze.
- [ ] Flutter Chrome tests.
- [ ] Flutter release web build.
- [ ] OpenAPI contract verification.
- [ ] Docker Compose topology/build/start/health.
- [ ] START-01–08 smoke.
- [ ] START-09–13 smoke.
- [ ] START-14–15 smoke.
- [ ] START-16 smoke.
- [ ] START-17 smoke.
- [ ] START-18–19 smoke.
- [ ] START-20 smoke.
- [ ] Profile/owner/locale/password smoke.
- [ ] Pre-START-22 business completion smoke.
- [ ] START-21 backup/recovery regression smoke.

The business-completion smoke must prove an actual public inquiry appearing in authenticated Contact Leads, lead workflow persistence and audit, Design Guide draft/publish/public delivery, and independent English/Hungarian CMS variants.
