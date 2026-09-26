#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central9-owner.txt"
HEADERS="$TMP_ROOT/himate-central9-headers.txt"
OUT="$TMP_ROOT/himate-central9-export.pdf"
rm -f "$OWNER_COOKIE" "$HEADERS" "$OUT"
trap 'rm -f "$OWNER_COOKIE" "$HEADERS" "$OUT"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-9 canonical package values are exact... '
plans="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$plans" | python3 -c '
import json,sys
d=json.load(sys.stdin); p={x["plan_key"]:x for x in d["items"]}
assert p["STARTER"]["monthly_price"]==990 and p["STARTER"]["module_limit"]==10
assert p["BUSINESS"]["monthly_price"]==1490 and p["BUSINESS"]["module_limit"]==20
assert p["FLEX"]["monthly_price"]==2490 and p["FLEX"]["selection_mode"]=="UNLIMITED"
'
echo ok

assert_pdf() {
  path="$1"
  rm -f "$HEADERS" "$OUT"
  curl -fsS -D "$HEADERS" -b "$OWNER_COOKIE" -o "$OUT" "$BASE_URL$path"
  grep -qi '^content-type: application/pdf' "$HEADERS"
  grep -qi '^content-disposition: attachment;' "$HEADERS"
  test -s "$OUT"
  test "$(dd if="$OUT" bs=1 count=5 2>/dev/null)" = "%PDF-"
}

printf 'CENTRAL-9 Partners PDF export... '
assert_pdf "/api/v1/partners/export.pdf"
echo ok
printf 'CENTRAL-9 Packages PDF export... '
assert_pdf "/api/v1/billing/packages/export.pdf"
echo ok
printf 'CENTRAL-9 Finance PDF export... '
assert_pdf "/api/v1/billing/finance/export.pdf"
echo ok
printf 'CENTRAL-9 Impact PDF export... '
assert_pdf "/api/v1/impact/export.pdf"
echo ok

printf 'CENTRAL-9 finance weekly series returns exactly four periods per currency... '
finance="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
printf '%s' "$finance" | python3 -c '
import json,sys
d=json.load(sys.stdin)
by={}
for row in d["weekly_paid"]: by.setdefault(row["currency"],[]).append(row)
assert by and all(len(rows)==4 for rows in by.values()), {k:len(v) for k,v in by.items()}
'
echo ok

echo 'CENTRAL-9 premium UI, performance, weekly report and PDF-only runtime acceptance passed'
