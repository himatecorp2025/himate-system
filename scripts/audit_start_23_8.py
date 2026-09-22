#!/usr/bin/env python3
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        raise SystemExit(f"START-23.8 audit failed: missing {path}")
    return p.read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.8 audit failed: " + message)

cms = read("services/cmd/cms/main.go")
design = read("services/cmd/cms/design.go")
seo = read("services/cmd/cms/seo.go")
gateway = read("services/cmd/gateway/main.go")
cms_ui = read("frontend/lib/cms_page.dart")
design_ui = read("frontend/lib/design_guide.dart")
partner_design_ui = read("frontend/lib/partner_design.dart")
partner_portal = read("services/cmd/gateway/partner_portal.go")
themes = read("services/cmd/cms/themes.go")
connector_main = read("services/cmd/connector/main.go")
website_adapter = read("services/cmd/connector/website_adapter.go")
seo_ui = read("frontend/lib/seo_panel.dart")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))
ci = read(".github/workflows/ci.yml")
compose = read("docker-compose.yml")
render = read("render.yaml")
openapi = read("docs/openapi.yaml")
smoke = read("scripts/smoke_start_23_8.sh")

# Release contract. Later START-23.x phases must preserve the START-23.8
# guarantees, so this historical guard accepts any release at or beyond 23.8.
def release_phase(text: str):
    match = re.search(r"0\\.8\\.\\d+-start-(23\\.\\d+)", text)
    require(match is not None, "START-23.x release identifier is missing")
    return tuple(int(x) for x in match.group(1).split("."))

require(release_phase(compose) >= (23, 8), "Compose release regressed below START-23.8")
require(release_phase(render) >= (23, 8), "Render release regressed below START-23.8")
require(release_phase(openapi) >= (23, 8), "OpenAPI release regressed below START-23.8")

# CMS full-page preview remains API-compatible while adding a real HTML route.
for token in (
    '"preview_path":"/preview/v1/cms/pages/"',
    '"preview_html_path":"/cms-preview/"',
    'mux.HandleFunc("/preview/v1/cms/design",a.previewDesign)',
    'mux.HandleFunc("/preview/v1/cms/design/media/",a.previewDesignMedia)',
    'start-23-8-design-preview-token',
    "preview_token_hash",
    "preview_token_issued_at",
):
    require(token in cms, f"CMS preview contract missing {token!r}")

for token in (
    "DESIGN_PREVIEW_CREATED",
    "expires_in_seconds",
    "1800",
    "designPreviewDraft",
    "time.Since(issuedAt.Time) > 30*time.Minute",
    "previewDesignMedia",
):
    require(token in design, f"Design preview backend missing {token!r}")

# Gateway arbitrary rendering and full HTML preview.
for token in (
    'mux.HandleFunc("/cms-preview/", a.cmsPagePreview)',
    'mux.HandleFunc("/design-preview", a.designPreview)',
    "dynamicCMSSectionHTML",
    "insertDynamicCMSSection",
    "himate-cms-dynamic",
    "renderPreviewCMSHTML",
    "serveDynamicCMSPage",
    "renderSiteDesignHTML",
    "fetchPublishedDesign",
    "fetchPreviewDesign",
    'w.Header().Set("X-Himate-Design", "published")',
):
    require(token in gateway, f"Gateway CMS/Design completion missing {token!r}")

require('strings.HasPrefix(r.URL.Path, "/cms-preview/")' not in gateway or "cmsPagePreview" in gateway,
        "full CMS preview route is not owned by the HTML renderer")
require('w.Header().Set("X-Frame-Options", "SAMEORIGIN")' in gateway,
        "Design preview lacks scoped SAMEORIGIN framing")
require("frame-ancestors 'self'" in gateway and "frame-ancestors 'none'" in gateway,
        "preview-specific and default frame-ancestor policies are not both present")
require('designErr == nil && design.Version > 0' in gateway,
        "public Design Guide is not gated on explicit publish")
require('html.EscapeString(section.Heading)' in gateway and 'html.EscapeString(section.Body)' in gateway,
        "dynamic CMS section text is not HTML-escaped")

# Multi-surface assets and tenant theme separation.
for token in (
    "header_wordmark",
    "footer_wordmark",
    "favicon",
    "app_icon",
    "login_logo",
    "email_logo",
    "designLayouts",
    "designAssetSlots",
):
    require(token in design or token in cms, f"Design asset/layout contract missing {token!r}")

for token in (
    "cms.design_profiles",
    "cms.design_scope_state",
    "PARTNER_THEME_ACTIVATED",
    '"content_binding": "UNCHANGED"',
    '"mechanics_binding": "UNCHANGED"',
    "owner_type='PARTNER'",
):
    require(token in themes or token in cms, f"Tenant theme contract missing {token!r}")

for token in (
    "/partner/api/v1/design",
    "design.read",
    "design.write",
    "partnerDesignMedia",
):
    require(token in partner_portal, f"Partner Portal design API missing {token!r}")

theme_audit_rule = 'return "PARTNER_THEME_ACTIVATED"'
module_audit_rule = 'return "PARTNER_MODULE_ACTIVATED"'
require(theme_audit_rule in partner_portal and module_audit_rule in partner_portal,
        "Partner Portal audit action catalog is incomplete")
require(partner_portal.index(theme_audit_rule) < partner_portal.index(module_audit_rule),
        "specific theme activation audit rule must precede generic module activation matching")

for token in (
    "Upload brand asset",
    "New custom design",
    "Content stays unchanged",
    "System logic stays unchanged",
    "/partner/api/v1/design/profiles",
    "/partner/api/v1/design/media",
):
    require(token in partner_design_ui, f"Partner Design UI missing {token!r}")

require("assets" in design_ui and "layout_key" in design_ui,
        "HIMATE Design Guide does not expose multi-surface assets and layout family")

brand_assets = read("frontend/lib/brand_assets.dart")
main_ui = read("frontend/lib/main.dart")
require("himateLoginWordmarkUrl" in brand_assets and "_loadPublishedBrandAssets" in main_ui,
        "published login-logo consumer is missing")
require("himateRuntimeIconUrl" in brand_assets and "assetUrl: himateLoginWordmarkUrl" in main_ui,
        "published app/login asset binding is missing")
require("publishedEmailLogoURL" in gateway and "multipart/alternative" in gateway,
        "published email-logo consumer is missing")

require("CMS_HOSTPORT" in connector_main and '"design":design' in website_adapter and "START-23.8" in website_adapter,
        "Website Adapter does not expose the active partner theme")
require("CMS_HOSTPORT: cms:10000" in compose,
        "Compose Connector is not bound to CMS theme state")
require("name: himate-connector" in render and "key: CMS_HOSTPORT" in render,
        "Render Connector is not bound to CMS theme state")

# Admin UI uses real previews instead of raw JSON.
for token in (
    "preview_html_path",
    "Full-page CMS preview created",
):
    require(token in cms_ui, f"CMS frontend full-page preview missing {token!r}")
for token in (
    "createPreview(String viewport)",
    "Desktop preview",
    "Tablet preview",
    "Mobile preview",
    "/api/v1/cms/design/preview",
):
    require(token in design_ui, f"Design frontend preview missing {token!r}")

# SEO publish must remain connected to initial HTML.
for token in (
    "enrichPublicSEO",
    "seoMap[\"keywords\"] = effectiveKeywords",
    'seoMap["json_ld"] = jsonLD',
):
    require(token in seo, f"CMS SEO enrichment missing {token!r}")
for token in (
    "renderGlobalSEOHTML",
    'data-himate-seo=\\\"organization\\\"',
    "page.SEO.JSONLD",
):
    require(token in gateway, f"Gateway SEO initial-HTML path missing {token!r}")
require("/api/v1/cms/seo/draft" in seo_ui and "/api/v1/cms/seo/publish" in seo_ui,
        "SEO admin mutation controls are not wired")

# OpenAPI routes.
for path in (
    "/api/v1/cms/pages:",
    "/api/v1/cms/pages/{pageId}/draft:",
    "/api/v1/cms/pages/{pageId}/preview:",
    "/api/v1/cms/pages/{pageId}/publish:",
    "/api/v1/cms/pages/{pageId}/rollback:",
    "/api/v1/cms/media:",
    "/api/v1/cms/design:",
    "/api/v1/cms/design/draft:",
    "/api/v1/cms/design/preview:",
    "/api/v1/cms/design/publish:",
    "/api/v1/cms/seo/draft:",
    "/api/v1/cms/seo/publish:",
    "/partner/api/v1/design:",
    "/partner/api/v1/design/media:",
    "/partner/api/v1/design/profiles:",
    "/partner/api/v1/design/profiles/{profileId}:",
    "/partner/api/v1/design/profiles/{profileId}/activate:",
    "/public/v1/cms/partner-design/{partnerId}:",
):
    require(path in openapi, f"OpenAPI missing {path}")

# Functional matrix closure.
completed = tuple(int(x) for x in str(matrix.get("completed_through", "0")).split("."))
require(completed >= (23, 8), "functional matrix is not completed through START-23.8")
rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") == "23.8"]
require(len(rows) == 17, f"expected 17 START-23.8 contracts, found {len(rows)}")
for row in rows:
    require(row.get("current_state") == "MUTATION_PROVEN_PROD_UNVERIFIED",
            f"{row.get('id')} is not mutation-proven")
    require("scripts/smoke_start_23_8.sh" in str(row.get("e2e_proof", "")),
            f"{row.get('id')} lacks START-23.8 E2E proof")

design_preview = next(x for x in rows if x.get("id") == "DESIGN-PREVIEW")
require(design_preview.get("api_pattern") == "/api/v1/cms/design/preview + /design-preview",
        "DESIGN-PREVIEW matrix route is not the real tokenized full-site preview")

# Acceptance and regression gates.
require("HIMATE START-23.8 CMS, Design & SEO Completion smoke passed" in smoke,
        "START-23.8 Compose proof is incomplete")
for token in (
    "audit_start_23_1_23_6.py",
    "audit_start_23_7.py",
    "audit_start_23_8.py",
    "smoke_start_23_8.sh",
    "docs/START-23.8_ACCEPTANCE.md",
):
    require(token in ci, f"CI does not retain required gate {token}")

print("START-23.8 static audit passed: 17/17 CMS, Design, SEO & tenant-theme contracts closed")
