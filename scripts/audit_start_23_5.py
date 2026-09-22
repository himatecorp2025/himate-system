#!/usr/bin/env python3
from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]

def read(path):
    p = ROOT / path
    if not p.exists():
        raise AssertionError(f"missing required file: {path}")
    return p.read_text()

common = read("services/internal/common/locale.go")
partners = read("services/cmd/partners/main.go")
catalog = read("services/cmd/catalog/main.go")
partner_portal_catalog = read("services/cmd/catalog/partner_portal.go")
impact = read("services/cmd/impact/main.go")
gateway = read("services/cmd/gateway/main.go")
frontend = read("frontend/lib/main.dart")
modules_ui = read("frontend/lib/module_control_plane.dart")
rbac_ui = read("frontend/lib/administration_rbac.dart")
ci = read(".github/workflows/ci.yml")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))

for token in ["RequestLocale", "NormalizeLocale", "Localized", "X-Himate-Locale", "Accept-Language"]:
    assert token in common, f"shared locale contract missing: {token}"

for token in ["name_en", "name_hu", "common.RequestLocale", "common.Localized"]:
    assert token in partners, f"partner category bilingual contract missing: {token}"

for token in ["label_en", "label_hu", "description_en", "description_hu", "common.RequestLocale", "common.Localized"]:
    assert token in catalog, f"catalog bilingual contract missing: {token}"
for token in ["label_en", "label_hu", "description_en", "description_hu", "common.RequestLocale", "common.Localized"]:
    assert token in partner_portal_catalog, f"Partner Portal bilingual catalog contract missing: {token}"

for token in ["label_en", "label_hu", "description_en", "description_hu", "common.RequestLocale", "common.Localized"]:
    assert token in impact, f"impact bilingual contract missing: {token}"

for token in ["identity.custom_roles", "label_en", "label_hu", "description_en", "description_hu", "common.RequestLocale"]:
    assert token in gateway, f"identity bilingual contract missing: {token}"

assert "'X-Himate-Locale': HimateI18n.activeLocale" in frontend
assert "api.clearCache();" in frontend
for token in ["'name_en'", "'name_hu'", "English category name *", "Hungarian category name *", "English display label *", "Hungarian display label *"]:
    assert token in frontend, f"frontend bilingual business editor missing: {token}"
for token in ["'label_en'", "'label_hu'", "'description_en'", "'description_hu'", "English group name *", "Hungarian group name *", "English module name *", "Hungarian module name *"]:
    assert token in modules_ui, f"module bilingual editor missing: {token}"
for token in ["'label_en'", "'label_hu'", "'description_en'", "'description_hu'", "English role name *", "Hungarian role name *"]:
    assert token in rbac_ui, f"role bilingual editor missing: {token}"

contracts = {x["id"]: x for x in matrix["contracts"]}
for cid in ["PART-CATEGORY-CREATE", "MODULE-CREATE-GROUP", "DYNAMIC-BILINGUAL-MODEL"]:
    item = contracts[cid]
    assert item["target_phase"] == "23.5", (cid, item["target_phase"])
    assert item["current_state"] == "MUTATION_PROVEN_PROD_UNVERIFIED", (cid, item["current_state"])
    assert "smoke_start_23_5.sh" in item["e2e_proof"], cid

assert matrix.get("completed_through") == "23.5"
assert "audit_start_23_5.py" in ci
assert "smoke_start_23_5.sh" in ci
print("HIMATE START-23.5 dynamic bilingual business-model static audit passed")
