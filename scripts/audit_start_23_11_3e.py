#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
branding = (root / "services/cmd/gateway/partner_branding.go").read_text(encoding="utf-8")
cms_main = (root / "services/cmd/cms/main.go").read_text(encoding="utf-8")
cms_themes = (root / "services/cmd/cms/themes.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("  Future<void> addPartner() async {")
end = frontend.index("  List<Map<String, dynamic>> get filtered => partners;", start)
add_partner = frontend[start:end]

dialog_index = add_partner.index("final ok = await showDialog<bool>(")
pre_dialog = add_partner[:dialog_index]
partner_page_tail = frontend[end:frontend.index("class PartnerWorkspace", end)]

checks = [
    (
        "New Partner action is wired to the Partners page button",
        "key: const Key('partners-new-partner-button')" in partner_page_tail
        and "onPressed: addPartner" in partner_page_tail,
    ),
    (
        "New Partner modal is created before any remote API dependency",
        "widget.api." not in pre_dialog
        and "final ok = await showDialog<bool>(" in add_partner
        and "key: const Key('new-partner-dialog')" in add_partner,
    ),
    (
        "New Partner modal is not blocked by Module Catalog",
        "widget.api.get('/api/v1/modules'" not in add_partner,
    ),
    ("New Partner keeps a safe category fallback", "'cat_006'" in add_partner and "Category service is still loading" in add_partner),
    ("company legal identity fields are collected", all(token in add_partner for token in [
        "registrationNumber", "taxId", "legalName", "brandName",
    ])),
    ("registered-office fields are collected", all(token in add_partner for token in [
        "stateRegion", "city", "postalCode", "addressLine1", "addressLine2",
    ])),
    ("operational contacts are collected", all(token in add_partner for token in [
        "financeContactEmail", "technicalContactEmail", "marketingContactEmail",
    ])),
    ("Partner Portal owner is created from onboarding", "/api/v1/partners/$partnerId/portal-users" in add_partner and "'role': 'owner'" in add_partner),
    ("partner logo can be selected and uploaded during onboarding", "Choose logo" in add_partner and "/api/v1/partners/$partnerId/logo" in add_partner and "'purpose': 'logo'" in add_partner),
    ("core partner creation stays PROSPECT and is not gated on invoice/provisioning", "'lifecycle': 'PROSPECT'" in add_partner and "activationInvoiceFile" not in add_partner and "/api/v1/provisioning/jobs" not in add_partner),
    ("supplementary setup failures do not erase the core partner", "final warnings = <String>[];" in add_partner and "Partner created. Supplementary setup needs attention" in add_partner),
    ("backend POST persists registration/tax/address/contact master data", all(token in partners for token in [
        'RegistrationNumber    string `json:"registration_number"`',
        'TaxID                 string `json:"tax_id"`',
        'AddressLine1          string `json:"address_line1"`',
        'FinanceContactEmail   string `json:"finance_contact_email"`',
        'MarketingContactEmail string `json:"marketing_contact_email"`',
    ])),
    ("gateway exposes partner logo upload orchestration", "func (a *app) adminPartnerLogo" in branding and 'strings.HasSuffix(r.URL.Path, "/logo")' in gateway),
    ("CMS stores explicit partner logo publication mapping", "cms.partner_brand_assets" in cms_main and "slot TEXT NOT NULL CHECK(slot IN ('logo'))" in cms_main),
    ("CMS partner media supports logo purpose and public URL", 'purpose == "logo"' in cms_themes and 'out["public_url"] = "/public/v1/cms/media/" + id' in cms_themes),
    ("OpenAPI documents partner logo upload", "/api/v1/partners/{partnerId}/logo:" in openapi),
    ("release version", "version: 0.8.22-start-23.11.3e" in openapi),
    ("render release version", "value: 0.8.22-start-23.11.3e" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for label in failures:
        print("FAIL:", label)
    raise SystemExit(1)

print("START-23.11.3e New Partner master-data onboarding static audit: PASS")
