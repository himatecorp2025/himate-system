#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113f-owner.txt"
rm -f "$OWNER_COOKIE"
trap 'rm -f "$OWNER_COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

owner_login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'partner category registry exposes all six canonical categories... '
categories="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partner-categories")"
python3 - "$categories" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
items=d["items"]
by_id={x["id"]:x for x in items}
expected={
  "cat_001":"Classical Music",
  "cat_002":"Fine Art",
  "cat_003":"Gallery",
  "cat_004":"Theatre",
  "cat_005":"Cultural Organization",
  "cat_006":"Other",
}
assert set(expected).issubset(by_id), by_id
for key,name in expected.items():
    assert by_id[key]["name_en"]==name, (key,by_id[key])
    assert by_id[key]["system"] is True
PY
echo ok

STAMP="$(date +%s)"
payload="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"Category QA "+stamp,
  "legal_name":"Category QA "+stamp+" LLC",
  "brand_name":"Category QA",
  "category_id":"cat_003",
  "lifecycle":"PROSPECT",
  "registration_number":"CAT-"+stamp,
  "tax_id":"CAT-TAX-"+stamp,
  "country":"United States",
  "city":"Test City",
  "postal_code":"10001",
  "address_line1":"1 Category Test Way",
  "contact_name":"Category Owner",
  "contact_email":"category."+stamp+"@himate.test"
}))
PY
)"

printf 'a non-Other category is accepted by normal partner creation... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners")"
python3 - "$partner" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["category_id"]=="cat_003", d
assert d["category_name"] in ("Gallery","Galéria"), d
PY
echo ok

echo 'HIMATE START-23.11.3f partner category resilience smoke passed'
