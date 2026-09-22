#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start235-owner.txt"
BODY="$TMP_ROOT/himate-start235-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"

payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"

printf 'START-23.5 owner login... '
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

category_en="Bilingual Foundation $STAMP"
category_hu="Kétnyelvű Alapítvány $STAMP"
printf 'create bilingual partner category and resolve locale variants... '
category="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -H 'X-Himate-Locale: en_US'   -d "$(python3 - "$category_en" "$category_hu" <<'PY'
import json,sys
print(json.dumps({"name_en":sys.argv[1],"name_hu":sys.argv[2]}))
PY
)" "$BASE_URL/api/v1/partner-categories")"
category_id="$(printf '%s' "$category" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["name"]==d["name_en"]; assert d["name_en"]!=d["name_hu"]; print(d["id"])')"
categories_en="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: en_US' "$BASE_URL/api/v1/partner-categories")"
categories_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/partner-categories")"
python3 - "$category_id" "$category_en" "$category_hu" "$categories_en" "$categories_hu" <<'PY'
import json,sys
cid,en,hu,enraw,huraw=sys.argv[1:]
a=next(x for x in json.loads(enraw)["items"] if x["id"]==cid)
b=next(x for x in json.loads(huraw)["items"] if x["id"]==cid)
assert a["name"]==en and b["name"]==hu,(a,b)
assert a["name_en"]==en and a["name_hu"]==hu
assert b["name_en"]==en and b["name_hu"]==hu
PY
echo ok

group_key="ci_235_$STAMP"
group_en="Bilingual Registry $STAMP"
group_hu="Kétnyelvű Katalógus $STAMP"
printf 'create bilingual module group and resolve locale variants... '
group_payload="$(python3 - "$group_key" "$group_en" "$group_hu" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label_en":sys.argv[2],"label_hu":sys.argv[3],"sort_order":235}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -H 'X-Himate-Locale: en_US' -d "$group_payload" "$BASE_URL/api/v1/module-groups" >/dev/null
groups_en="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: en_US' "$BASE_URL/api/v1/module-groups")"
groups_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/module-groups")"
python3 - "$group_key" "$group_en" "$group_hu" "$groups_en" "$groups_hu" <<'PY'
import json,sys
key,en,hu,enraw,huraw=sys.argv[1:]
a=next(x for x in json.loads(enraw)["items"] if x["group_key"]==key)
b=next(x for x in json.loads(huraw)["items"] if x["group_key"]==key)
assert a["label"]==en and b["label"]==hu,(a,b)
assert a["label_en"]==en and a["label_hu"]==hu
PY
echo ok

module_key="ci.bilingual_$STAMP"
module_en="Bilingual Module $STAMP"
module_hu="Kétnyelvű Modul $STAMP"
desc_en="English acceptance description $STAMP"
desc_hu="Magyar elfogadási leírás $STAMP"
printf 'create bilingual module and verify registry plus commercial matrix locale... '
module_payload="$(python3 - "$module_key" "$group_key" "$module_en" "$module_hu" "$desc_en" "$desc_hu" <<'PY'
import json,sys
key,group,en,hu,den,dhu=sys.argv[1:]
print(json.dumps({
 "key":key,"group_key":group,"label_en":en,"label_hu":hu,
 "description_en":den,"description_hu":dhu,
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":35,"default_activation_fee":90,
 "availability":"ACTIVE","module_type":"FEATURE","owner_team":"Platform",
 "source_repository":"himatecorp2025/himate-system","source_path":"modules/ci/bilingual",
 "source_ref":"develop","source_commit":"start235","artifact_type":"docker",
 "artifact_reference":"sha256:start235","min_platform_version":"0.8.9",
 "manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -H 'X-Himate-Locale: en_US' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null
modules_en="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: en_US' "$BASE_URL/api/v1/modules")"
modules_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/modules")"
python3 - "$module_key" "$module_en" "$module_hu" "$desc_en" "$desc_hu" "$modules_en" "$modules_hu" <<'PY'
import json,sys
key,en,hu,den,dhu,enraw,huraw=sys.argv[1:]
a=next(x for x in json.loads(enraw)["items"] if x["key"]==key)
b=next(x for x in json.loads(huraw)["items"] if x["key"]==key)
assert a["label"]==en and b["label"]==hu,(a,b)
assert a["description"]==den and b["description"]==dhu,(a,b)
assert a["label_en"]==en and a["label_hu"]==hu
assert a["description_en"]==den and a["description_hu"]==dhu
PY
matrix_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/module-commercial-matrix?partner_ids=ptr_000001")"
python3 - "$module_key" "$module_hu" "$group_hu" "$matrix_hu" <<'PY'
import json,sys
key,label,group,raw=sys.argv[1:]
x=next(i for i in json.loads(raw)["items"] if i["key"]==key)
assert x["label"]==label,x
assert x["group_label"]==group,x
PY
echo ok

metric_key="ci.bilingual.$STAMP"
metric_en="Bilingual Attendance $STAMP"
metric_hu="Kétnyelvű Látogatottság $STAMP"
metric_desc_en="English metric description $STAMP"
metric_desc_hu="Magyar mérőszám-leírás $STAMP"
printf 'create bilingual impact definition and resolve definition/summary locale... '
metric_payload="$(python3 - "$metric_key" "$metric_en" "$metric_hu" "$metric_desc_en" "$metric_desc_hu" <<'PY'
import json,sys
key,en,hu,den,dhu=sys.argv[1:]
print(json.dumps({"metric_key":key,"label_en":en,"label_hu":hu,"description_en":den,"description_hu":dhu,"unit":"count","aggregation":"SUM","scope":"PARTNER"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$metric_payload" "$BASE_URL/api/v1/impact/definitions" >/dev/null
defs_en="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: en_US' "$BASE_URL/api/v1/impact/definitions")"
defs_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/impact/definitions")"
python3 - "$metric_key" "$metric_en" "$metric_hu" "$metric_desc_en" "$metric_desc_hu" "$defs_en" "$defs_hu" <<'PY'
import json,sys
key,en,hu,den,dhu,enraw,huraw=sys.argv[1:]
a=next(x for x in json.loads(enraw)["items"] if x["metric_key"]==key)
b=next(x for x in json.loads(huraw)["items"] if x["metric_key"]==key)
assert a["label"]==en and b["label"]==hu
assert a["description"]==den and b["description"]==dhu
PY
value_payload="$(python3 - "$metric_key" "$TODAY" <<'PY'
import json,sys
print(json.dumps({"partner_id":"ptr_000001","metric_key":sys.argv[1],"period_start":sys.argv[2],"period_end":sys.argv[2],"numeric_value":7,"provenance":"MANUAL","source_ref":"start-23.5-smoke"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$value_payload" "$BASE_URL/api/v1/impact/values" >/dev/null
summary_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/impact/summary?partner_id=ptr_000001")"
python3 - "$metric_key" "$metric_hu" "$summary_hu" <<'PY'
import json,sys
key,label,raw=sys.argv[1:]
x=next(i for i in json.loads(raw)["items"] if i["metric_key"]==key)
assert x["label"]==label,x
assert float(x["numeric_value"])==7,x
PY
echo ok

role_key="ci_bilingual_$STAMP"
role_en="Bilingual Viewer $STAMP"
role_hu="Kétnyelvű Megtekintő $STAMP"
role_desc_en="English role description $STAMP"
role_desc_hu="Magyar szerepkör-leírás $STAMP"
printf 'create and update bilingual custom role without changing permission key... '
role_payload="$(python3 - "$role_key" "$role_en" "$role_hu" "$role_desc_en" "$role_desc_hu" <<'PY'
import json,sys
key,en,hu,den,dhu=sys.argv[1:]
print(json.dumps({"key":key,"label_en":en,"label_hu":hu,"description_en":den,"description_hu":dhu,"permissions":["catalog.read"]}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$role_payload" "$BASE_URL/api/v1/admin/roles" >/dev/null
roles_en="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: en_US' "$BASE_URL/api/v1/admin/roles")"
roles_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/admin/roles")"
python3 - "$role_key" "$role_en" "$role_hu" "$roles_en" "$roles_hu" <<'PY'
import json,sys
key,en,hu,enraw,huraw=sys.argv[1:]
a=next(x for x in json.loads(enraw)["items"] if x["key"]==key)
b=next(x for x in json.loads(huraw)["items"] if x["key"]==key)
assert a["label"]==en and b["label"]==hu
assert a["permissions"]==["catalog.read"] and b["permissions"]==["catalog.read"]
PY
updated_hu="$role_hu frissített"
patch_payload="$(python3 - "$updated_hu" <<'PY'
import json,sys
print(json.dumps({"label_hu":sys.argv[1],"permissions":["catalog.read"]}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$patch_payload" "$BASE_URL/api/v1/admin/roles/$role_key" >/dev/null
roles_hu="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/admin/roles")"
python3 - "$role_key" "$updated_hu" "$roles_hu" <<'PY'
import json,sys
key,hu,raw=sys.argv[1:]
x=next(i for i in json.loads(raw)["items"] if i["key"]==key)
assert x["label"]==hu,x
assert x["permissions"]==["catalog.read"],x
PY
echo ok

echo "HIMATE START-23.5 dynamic bilingual business-model smoke passed"
