#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("class _PartnersPageState")
end = frontend.index("class PartnerWorkspace", start)
partner_page = frontend[start:end]

required_ids = ["cat_001", "cat_002", "cat_003", "cat_004", "cat_005", "cat_006"]
required_names = ["Classical Music", "Fine Art", "Gallery", "Theatre", "Cultural Organization", "Other"]

checks = [
    (
        "frontend embeds the complete built-in category catalog",
        all(token in partner_page for token in required_ids)
        and all(name in partner_page for name in required_names),
    ),
    (
        "New Partner never falls back to Other-only",
        "<String, dynamic>{'id': 'cat_006', 'name': 'Other'}" not in partner_page,
    ),
    (
        "live category registry is merged on top of built-ins",
        "_mergePartnerCategories" in partner_page
        and "byID[id] = Map<String, dynamic>.from(item)" in partner_page,
    ),
    (
        "New Partner refreshes the live category registry without blocking the modal",
        "widget.api.get('/api/v1/partner-categories', force: true)" in partner_page
        and "categoryRefreshStarted" in partner_page,
    ),
    (
        "legacy misleading loading message is removed",
        "Category service is still loading" not in partner_page,
    ),
    (
        "partners service still seeds the same six canonical categories",
        all(name in partners for name in required_names)
        and 'fmt.Sprintf("cat_%03d", i+1)' in partners,
    ),
    ("release version", "version: 0.8.33-start-23.12" in openapi),
    ("render release version", "value: 0.8.33-start-23.12" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:", failure)
    raise SystemExit(1)

print("START-23.11.3f partner category resilience static audit: PASS")
