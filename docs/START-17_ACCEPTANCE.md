# START-17 Acceptance — Public CMS, SSR and SEO

## Required behavior
- [x] Public marketing routes consume only active PUBLISHED CMS data.
- [x] CMS title/meta/canonical are present in the initial HTML response.
- [x] Open Graph title/description/url/image are rendered server-side.
- [x] noindex state is rendered in robots metadata.
- [x] Hidden CMS sections are removed server-side.
- [x] Visible CMS heading/body/CTA/media are rendered before JavaScript.
- [x] Client JavaScript remains hydration/fallback only.
- [x] Sitemap is generated from the published manifest.
- [x] robots.txt advertises the sitemap.
- [x] CMS failure has a bounded static fallback rather than taking down the public page.

## Evidence
- `services/cmd/gateway/main.go` — public CMS fetch + server rendering
- `services/cmd/cms/main.go` — visible/hidden public contract
- `frontend/web/site.js` — hydration/fallback
- `scripts/smoke_start_17.sh`
- `TestRenderPublishedCMSHTML`

START-17 is complete only when the PR Compose job passes the START-17 smoke.
