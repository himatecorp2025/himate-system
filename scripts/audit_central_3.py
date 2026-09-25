#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

frontend = read("frontend/lib/main.dart")
partners = read("services/cmd/partners/main.go")
localization = read("frontend/lib/localization.dart")

for token in [
    "bool referenceOnly = false;",
    "if (referenceOnly) params['reference'] = 'true';",
    "void applyPortfolioPreset({String lifecycle = 'ALL', bool reference = false})",
    "onTap: () => applyPortfolioPreset()",
    "onTap: () => applyPortfolioPreset(lifecycle: 'LIVE')",
    "onTap: () => applyPortfolioPreset(lifecycle: 'PROSPECT')",
    "onTap: () => applyPortfolioPreset(reference: true)",
    "Reference partners only",
    "portfolioLifecycleCounts",
    "_portfolioStatsUri()",
]:
    require(token in frontend, f"Partners KPI/filter contract missing: {token}")

require('referenceOnly, _ := strconv.ParseBool(r.URL.Query().Get("reference"))' in partners,
        "Partners API does not parse reference filter")
require("p.reference_partner=TRUE" in partners,
        "Partners API does not apply authoritative reference-partner filter")

partners_get = partners[partners.index("func (a *app) partners"):partners.index("func canTransition")]
require("p.test_partner=FALSE" not in partners_get and "p.test_partner = FALSE" not in partners_get,
        "Partner list must not hide Golden/Test partners")
require("if (p['test_partner'] == true)" in frontend and "_StatusPill(label: 'TEST')" in frontend,
        "Partner cards do not visibly identify Test Partners")

require('"category_name_en": p.CategoryNameEN' in partners and '"category_name_hu": p.CategoryNameHU' in partners,
        "Partners API does not expose bilingual category names")
require("String _localizedPartnerCategory(Map<String, dynamic> partner)" in frontend,
        "Partner UI does not select category label by active locale")
require("_localizedPartnerCategory(partner)" in frontend and "_localizedPartnerCategory(p)" in frontend,
        "Partner workspace/card do not use localized category names")

for token in [
    "'/app/partners': (_) =>",
    "initialSelected: 1",
    "tooltip: uiLiteral('Back to Partners')",
    "final VoidCallback? onBack;",
    "onPressed: widget.onBack ?? () => Navigator.pop(context)",
    "pushNamedAndRemoveUntil('/app/partners', (route) => false)",
]:
    require(token in frontend, f"Partners parent-navigation contract missing: {token}")

technical_literals = {"USD","EUR","GBP","PRODUCTION","STAGING","YYYY-MM-DD","example.com"}
start = frontend.index("class PartnersPage")
end = frontend.index("class PackagesPage", start)
scope = frontend[start:end]
card_start = frontend.index("class PartnerCard")
card_end = frontend.index("class _PartnerLogo", card_start)
scope += frontend[card_start:card_end]

patterns = [
    r"LText\(\s*'([^']+)'",
    r"uiLiteral\(\s*'([^']+)'",
    r"\btitle:\s*'([^']+)'",
    r"\bsubtitle:\s*'([^']+)'",
    r"\bprimaryLabel:\s*'([^']+)'",
]
fixed = set()
for pattern in patterns:
    for value in re.findall(pattern, scope):
        if "$" not in value:
            fixed.add(value)

literal_block = localization[localization.index("static const Map<String, String> _literalHu"):]
missing = sorted(value for value in fixed if value not in technical_literals and f"'{value}':" not in literal_block)
require(not missing, "Partners bilingual literal coverage missing: " + ", ".join(missing))

for token in [
    "uiLiteral('Previous partner onboarding could not be resumed yet')",
    "uiLiteral('Partner could not be created')",
    "uiLiteral('Provisioning could not complete')",
    "uiLiteral('Portal user could not be created')",
    "uiLiteral('Portal user could not be updated')",
    "uiLiteral('Credential operation failed')",
]:
    require(token in frontend, f"Dynamic partner message is not localized: {token}")

print("Central-3 Partners/navigation/bilingual acceptance: PASS")
