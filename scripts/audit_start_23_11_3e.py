#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
durability = (root / "services/cmd/gateway/phase2_durability.go").read_text(encoding="utf-8")
branding = (root / "services/cmd/gateway/partner_branding.go").read_text(encoding="utf-8")
cms_main = (root / "services/cmd/cms/main.go").read_text(encoding="utf-8")
cms_themes = (root / "services/cmd/cms/themes.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("  Future<void> addPartner() async {")
end = frontend.index("  List<Map<String, dynamic>> get filtered => partners;", start)
add_partner = frontend[start:end]

dialog_index = add_partner.index("final createdResult = await showDialog<Map<String, dynamic>>(")
pre_dialog = add_partner[:dialog_index]
partner_page_tail = frontend[end:frontend.index("class PartnerWorkspace", end)]

checks = [
    (
        "New Partner action is wired to the Partners page button",
        "key: const Key('partners-new-partner-button')" in partner_page_tail
        and "onPressed: addPartner" in partner_page_tail,
    ),
    (
        "New Partner modal is not blocked by any awaited remote dependency",
        "await widget.api." not in pre_dialog
        and "await _loadCategories" not in pre_dialog
        and "final createdResult = await showDialog<Map<String, dynamic>>(" in add_partner
        and "key: const Key('new-partner-dialog')" in add_partner,
    ),
    (
        "New Partner modal is not blocked by Module Catalog",
        "widget.api.get('/api/v1/modules'" not in add_partner,
    ),
    ("New Partner keeps the complete built-in category catalog available", all(token in frontend for token in ["cat_001","cat_002","cat_003","cat_004","cat_005","cat_006"]) and "_mergePartnerCategories(categories)" in add_partner and "Category service is still loading" not in add_partner),
    ("company legal identity fields are collected", all(token in add_partner for token in [
        "registrationNumber", "taxId", "legalName", "brandName",
    ])),
    ("registered-office fields are collected", all(token in add_partner for token in [
        "stateRegion", "city", "postalCode", "addressLine1", "addressLine2",
    ])),
    ("operational contacts are collected", all(token in add_partner for token in [
        "financeContactEmail", "technicalContactEmail", "marketingContactEmail",
    ])),
    ("Partner Portal owner is created from durable onboarding",
     "/api/v1/partner-onboarding" in add_partner
     and "'portal_owner': {" in add_partner
     and "func (a *app) ensureOnboardingOwner" in durability
     and "'owner'" in durability),
    ("partner logo can be selected and uploaded during onboarding", "Choose logo" in add_partner and "/api/v1/partners/$partnerId/logo" in add_partner and "'purpose': 'logo'" in add_partner),
    ("core partner creation stays PROSPECT and is not gated on invoice/provisioning", "'lifecycle': 'PROSPECT'" in add_partner and "activationInvoiceFile" not in add_partner and "/api/v1/provisioning/jobs" not in add_partner),
    ("validation and backend errors stay inside the New Partner modal",
        "String? formError;" in add_partner
        and "Partner registration needs attention" in add_partner
        and "This window will stay open." in add_partner
        and "Navigator.pop<Map<String, dynamic>>(dialogContext" in add_partner),
    ("partial onboarding resumes against the same durable saga instead of creating duplicates",
        "himate_pending_partner_onboarding" in add_partner
        and "/api/v1/partner-onboarding/$pendingRequestId/resume" in add_partner
        and "identity.partner_onboarding_sagas" in durability
        and 'payload["onboarding_request_id"]=s.RequestID' in durability),
    ("backend distinguishes malformed JSON/version skew from an empty display name",
        'Invalid partner request: "+err.Error()' in partners
        and 'Display name is required' in partners
        and 'common.Decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == ""' not in partners),
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
    ("release version", "version: 0.8.32-start-23.11.7" in openapi),
    ("render release version", "value: 0.8.32-start-23.11.7" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for label in failures:
        print("FAIL:", label)
    raise SystemExit(1)

print("START-23.11.3e New Partner master-data onboarding static audit: PASS")
