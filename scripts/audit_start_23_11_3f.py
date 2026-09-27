#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
gateway_c10 = (root / "services/cmd/gateway/central10.go").read_text(encoding="utf-8")
gateway_step4 = (root / "services/cmd/gateway/central_step4_snapshots.go").read_text(encoding="utf-8")
gateway_snapshots = (root / "services/cmd/gateway/central_step3_snapshots.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

start = frontend.index("class _PartnersPageState")
end = frontend.index("class PartnerWorkspace", start)
partner_page = frontend[start:end]

required_ids = ["cat_001", "cat_002", "cat_003", "cat_004", "cat_005", "cat_006"]
required_names = ["Classical Music", "Fine Art", "Gallery", "Theatre", "Cultural Organization", "Other"]

checks = [
    (
        "Go Central read model embeds the complete built-in category catalog",
        all(token in gateway_c10 for token in required_ids)
        and all(name in gateway_c10 for name in required_names)
        and "central10PartnerCategories" in gateway_c10,
    ),
    (
        "New Partner never falls back to Other-only",
        "<String, dynamic>{'id': 'cat_006', 'name': 'Other'}" not in partner_page,
    ),
    (
        "materialized category registry is merged on top of Go built-ins",
        "byID[id] = row" in gateway_c10
        and '"categories": central10PartnerCategories(common.RequestLocale(r), rawCategories)' in gateway_c10
        and 'rawCategories := step4Items(snapshot["categories_raw"])' in gateway_c10
        and 'categoriesErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partner-categories", &categories)' in gateway_step4
        and '"categories_raw": categories.Items' in gateway_step4,
    ),
    (
        "New Partner refreshes the Central category read model without blocking the modal",
        "/api/v1/central/partners?limit=1&offset=0" in partner_page
        and "response['categories']" in partner_page
        and "categoryRefreshStarted" in partner_page,
    ),
    (
        "category refresh failure retains the persisted Last-Known-Good Partners snapshot",
        '"partner_categories"' in gateway_step4
        and 'status = "partial"' in gateway_step4
        and 'centralSnapshotValid(key, payload)' in gateway_snapshots
        and '"central read-model refresh rejected; retaining last-known-good snapshot"' in gateway_snapshots,
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
