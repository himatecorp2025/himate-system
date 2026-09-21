# Pre-START-22 Business Completion Acceptance

This release gate closes the four business-completeness gaps identified after the START-01–20 audit. It does not redefine START-21 backups/recovery or begin the next numbered START block.

**Release evidence:** code candidate `71021f43639784ae62245ebcb61fedddfe3a2277` · GitHub Actions run `35604704099` · Go, Flutter and full Docker Compose/START regression gates all passed.

## 1. SEO complexity / SEO & Keywords

- [x] Website & Marketing exposes a dedicated responsive SEO & Keywords workspace.
- [x] Global keywords are managed independently for English and Hungarian.
- [x] Each CMS page version supports its own page-specific keyword set.
- [x] Global and page-specific keywords are normalized, deduplicated and bounded by backend validation.
- [x] SEO settings use explicit DRAFT and PUBLISHED states with version metadata.
- [x] Saving and publishing global SEO settings is audited.
- [x] Automatic SEO analysis scores each CMS draft for title length, meta-description length, canonical validity, keyword count, visible-content depth and keyword/content fit.
- [x] Automatic analysis produces deterministic keyword suggestions from visible page content.
- [x] Public CMS output merges language-specific global keywords with page keywords.
- [x] Gateway SSR emits a server-rendered meta keywords tag for published CMS pages.
- [x] Public CMS output includes Schema.org WebPage JSON-LD with language and publisher defaults.
- [x] Published English/Hungarian variants expose canonical alternate mappings and Gateway SSR emits hreflang links.
- [x] A configurable default Open Graph image may be used when a page has no explicit OG image.
- [x] Existing title, meta description, canonical, Open Graph, noindex, sitemap and robots behavior remains intact.

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
- [x] Core Partner, Finance, Impact, Evidence, Reports, CMS, Operations, Backups, Administration, Contact Leads, SEO & Keywords and Design Guide terminology has Hungarian coverage.
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

## Additional security hardening completed in the same correction

- [x] All newly accepted passwords require at least 12 characters.
- [x] A password must contain at least one lowercase letter, uppercase letter, number and special/punctuation character.
- [x] The rule is authoritative in the Gateway backend and mirrored by the Flutter Profile/Administration surfaces.
- [x] Bootstrap owner, profile password changes and administrator creation/replacement use the same policy.
- [x] Existing password/session rotation and secret-redaction behavior remains unchanged.

## Regression and release gates

This correction is accepted only when the branch CI proves all of the following on the same candidate commit:

- [x] Go vet.
- [x] Go unit tests.
- [x] Go race tests.
- [x] Go build for all commands.
- [x] Flutter analyze.
- [x] Flutter Chrome tests.
- [x] Flutter release web build.
- [x] OpenAPI contract verification.
- [x] Docker Compose topology/build/start/health.
- [x] START-01–08 smoke.
- [x] START-09–13 smoke.
- [x] START-14–15 smoke.
- [x] START-16 smoke.
- [x] START-17 smoke.
- [x] START-18–19 smoke.
- [x] START-20 smoke.
- [x] Profile/owner/locale/password smoke.
- [x] Pre-START-22 business completion smoke.
- [x] START-21 backup/recovery regression smoke.

The business-completion smoke proves global SEO draft/publish/public delivery, safe publication of the default Open Graph media asset, global SEO injection into source-controlled fallback pages, combined bilingual page/global keywords, automatic SEO audit, structured SEO output, an actual public inquiry appearing in authenticated Contact Leads, lead workflow persistence and audit, Design Guide draft/publish/public delivery, and independent published English/Hungarian CMS variants.
