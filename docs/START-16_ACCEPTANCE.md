# START-16 Acceptance — HIMATE CMS

This document is the release gate for START-16. START-17 public landing-page CMS rendering is explicitly out of scope: the already approved public landing page and subpages remain visually unchanged in this block.

## Functional acceptance

- [x] CMS is implemented as a dedicated containerized Go microservice.
- [x] CMS owns a separate PostgreSQL schema and service-scoped migrations.
- [x] Marketing content can be managed from the existing Website & Marketing admin area without source-code edits.
- [x] Content supports heading, body, media asset ID, CTA label/URL, visibility and deterministic section order.
- [x] Component types are predefined and backend-validated.
- [x] Page SEO supports title, meta description, canonical URL, Open Graph title/description/image, noindex and slug.
- [x] Slug and canonical rules are validated by the backend.
- [x] Duplicate published slug/canonical conflicts are rejected.
- [x] CMS media uses stable media asset IDs rather than arbitrary public file paths.
- [x] CMS media is persisted through the shared Storage service, not container-local persistent state.
- [x] Media uploads are bounded to 10 MiB files / 11 MiB multipart requests.
- [x] Media type is content-sniffed; only PNG, JPEG and WebP are accepted.
- [x] Stored media receives SHA-256 integrity metadata and object checksums are cross-checked.
- [x] Media object extension is derived from detected content type rather than the uploaded filename.
- [x] Every page content modification creates a new immutable version.
- [x] Page content follows the DRAFT → PREVIEW → PUBLISHED workflow.
- [x] Publishing requires a fresh preview of the current draft; stale previews cannot be published.
- [x] Preview tokens are cryptographically random, persisted only as SHA-256 and returned raw only on issue/rotation.
- [x] Preview content is no-store/noindex and available only through the scoped token.
- [x] Preview media uses the public slug plus scoped preview token, so visual preview requires no internal page identifier.
- [x] Public CMS page APIs expose only the active PUBLISHED version.
- [x] Public/preview CMS payloads expose only allowlisted content fields and do not leak internal actor, page, source-version or rollback metadata.
- [x] Draft or preview content cannot leak through public page APIs.
- [x] Disabled sections retain their stored content while being removed from public/preview output.
- [x] Preview and production use the same content model version.
- [x] Publish validation rejects incomplete SEO/visible-section content.
- [x] Media is public only when referenced by the active published version.
- [x] Prior PUBLISHED versions can be restored by rollback.
- [x] Rollback creates a new PUBLISHED version and explicitly records rollback_of_version_id.
- [x] CMS page create/draft/preview/publish/rollback operations generate CMS-local audit events with actor and correlation ID.
- [x] Admin can inspect version history and CMS-local audit history.
- [x] Public manifest exposes published slug/canonical/title/noindex/version metadata for START-17 sitemap/robots preparation.
- [x] The Flutter CMS workspace is responsive and uses the existing approved admin design system.
- [x] Existing public landing/subpage HTML/CSS/visual design is not rebuilt or replaced in START-16.
- [x] START-17 remains responsible for binding the existing public pages to published CMS content and SEO-compatible HTML rendering.

## Cross-cutting release gates

- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go build ./cmd/...`
- [ ] `flutter analyze`
- [ ] `flutter test --platform chrome`
- [ ] `flutter build web --release`
- [ ] Render Blueprint validation includes `himate-cms` and `CMS_HOSTPORT`.
- [ ] Docker Compose topology validates.
- [ ] All microservice containers build and start.
- [ ] Gateway/private-service health includes CMS.
- [ ] START-01–08 regression smoke passes.
- [ ] START-09–13 integration smoke passes.
- [ ] START-14–15 integration smoke passes.
- [ ] START-16 CMS integration smoke passes.

Merge policy: PR #8 may merge to `develop` only after every release gate above is green. A full post-merge `develop` run is mandatory before START-17 begins.
