#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("  Future<void> addPartner() async {")
end = frontend.index("  List<Map<String, dynamic>> get filtered => partners;", start)
add_partner = frontend[start:end]

dialog_start = add_partner.index("final createdResult = await showDialog<Map<String, dynamic>>(")
partner_post = add_partner.index("widget.api.post('/api/v1/partners'")
success_pop = add_partner.index("Navigator.pop<Map<String, dynamic>>(dialogContext, created")

checks = [
    (
        "New Partner modal owns validation and API errors",
        "String? formError;" in add_partner
        and "Partner registration needs attention" in add_partner
        and "Registration is not complete yet" in add_partner,
    ),
    (
        "validation errors are shown inside the modal instead of a page snackbar",
        "validationMessage" in add_partner
        and "formError = validationMessage;" in add_partner
        and "ScaffoldMessenger.of(context).showSnackBar" not in add_partner[dialog_start:partner_post],
    ),
    (
        "modal remains open until authoritative partner creation/setup succeeds",
        partner_post < success_pop
        and "This window will stay open." in add_partner,
    ),
    (
        "partial setup retries the same partner rather than creating a duplicate",
        "String? stagedPartnerId;" in add_partner
        and "Retry setup" in add_partner
        and "widget.api.patch(" in add_partner,
    ),
    (
        "backend JSON decode errors are not mislabeled as display-name errors",
        'if err := common.Decode(r, &in); err != nil {' in partners
        and 'Invalid partner request: "+err.Error()' in partners
        and 'common.Decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == ""' not in partners,
    ),
    (
        "empty display name still has its own precise validation",
        'if in.DisplayName == "" {' in partners
        and '"Display name is required"' in partners,
    ),
    (
        "primary-domain conflicts remain explicit while display names may repeat",
        'PRIMARY_DOMAIN_EXISTS' in partners
        and 'partnerTechnicalSlug(in.DisplayName, id)' in partners,
    ),
    ("release version", "version: 0.8.31-start-23.11.6" in openapi),
    ("render release version", "value: 0.8.31-start-23.11.6" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:", failure)
    raise SystemExit(1)

print("START-23.11.3g partner registration error-handling static audit: PASS")
