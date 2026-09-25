#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
partner_tests = (root / "services/cmd/partners/main_test.go").read_text(encoding="utf-8")
durability = (root / "services/cmd/gateway/phase2_durability.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("  Future<void> addPartner() async {")
end = frontend.index("  List<Map<String, dynamic>> get filtered => partners;", start)
add_partner = frontend[start:end]

checks = [
    (
        "display name is presentation data, not the unique technical key",
        "Display names do not need to be unique." in add_partner
        and "partnerTechnicalSlug(in.DisplayName, id)" in partners
        and "display-name slug or primary domain is already in use" not in partners,
    ),
    (
        "technical slugs remain unique for duplicate display names",
        "func partnerTechnicalSlug(displayName, partnerID string) string" in partners
        and "TestPartnerTechnicalSlugAllowsDuplicateDisplayNames" in partner_tests,
    ),
    (
        "core partner POST is idempotent across a lost client response",
        "onboarding_request_id" in add_partner
        and "onboarding_request_id" in partners
        and "partners_onboarding_request_unique" in partners
        and "SELECT id FROM partners.partners WHERE onboarding_request_id=$1" in partners,
    ),
    (
        "partial onboarding cannot be dismissed while incomplete",
        "PopScope(" in add_partner
        and "canPop: completionReady || canDismiss" in add_partner
        and "dismissEnabled: canDismiss" in add_partner
        and "onDismiss: () => Navigator.pop<Map<String, dynamic>>(dialogContext)" in add_partner,
    ),
    (
        "Partner Portal owner retry reconciles server state before insert",
        "func (a *app) ensureOnboardingOwner" in durability
        and "SELECT id,partner_id,role_key,active FROM identity.partner_users" in durability
        and "existingPartner==s.PartnerID" in durability,
    ),
    (
        "logo retry recognizes an already committed partner logo",
        "created['logo_url']" in add_partner
        and "logoUploaded = true;" in add_partner,
    ),
    (
        "commercial defaults retry reconciles persisted terms before PUT",
        "onboardingTermsMatch" in durability
        and "billing terms readback" in durability
        and "http.MethodGet" in durability
        and "http.MethodPut" in durability,
    ),
    (
        "primary-domain uniqueness remains explicit and independent of display name",
        '"PRIMARY_DOMAIN_EXISTS"' in partners
        and "Primary domain is already assigned to another partner" in partners,
    ),
    (
        "OpenAPI matches idempotent duplicate-name onboarding contract",
        "duplicate display names are allowed" in openapi
        and "onboarding_request_id:" in openapi
        and "Existing partner returned for an idempotent onboarding replay" in openapi
        and "Display-name-derived slug or primary domain is already in use" not in openapi,
    ),
    ("release version", "version: 0.8.33-start-23.12" in openapi),
    ("render release version", "value: 0.8.33-start-23.12" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:", failure)
    raise SystemExit(1)

print("START-23.11.3h Test Partner Onboarding Hardening static audit: PASS")
