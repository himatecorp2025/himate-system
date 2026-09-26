#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central10-owner.txt"
BODY="$TMP_ROOT/himate-central10-body.json"
rm -f "$OWNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

SEQ=0
assert_fast_read_model() {
  path="$1"
  name="$2"
  SEQ=$((SEQ + 1))
  file="$TMP_ROOT/himate-central10-$SEQ.json"
  rm -f "$file"
  elapsed="$(curl -fsS -b "$OWNER_COOKIE" -o "$file" -w '%{time_total}' "$BASE_URL$path")"
  python3 - "$file" "$elapsed" "$name" <<'PY'
import json,sys
path,elapsed,name=sys.argv[1],float(sys.argv[2]),sys.argv[3]
with open(path,encoding="utf-8") as f:
    data=json.load(f)
assert elapsed < 0.800, f"{name} first response exceeded 800 ms: {elapsed*1000:.1f} ms"
meta=data.get("meta") or {}
assert meta.get("architecture")=="GO_BACKEND_READ_MODEL", (name,meta)
assert meta.get("frontend_role")=="PRESENTATION_ONLY", (name,meta)
assert int(meta.get("target_first_usable_data_ms",0))==800, (name,meta)
assert int(meta.get("backend_budget_ms",9999))<=650, (name,meta)
assert int(meta.get("duration_ms",9999))<800, (name,meta)
print(f"{name}: {elapsed*1000:.1f} ms HTTP / {meta.get('duration_ms')} ms Go")
PY
  cp "$file" "$BODY"
}

assert_fast_json() {
  path="$1"
  name="$2"
  SEQ=$((SEQ + 1))
  file="$TMP_ROOT/himate-central10-$SEQ.json"
  rm -f "$file"
  elapsed="$(curl -fsS -b "$OWNER_COOKIE" -o "$file" -w '%{time_total}' "$BASE_URL$path")"
  python3 - "$file" "$elapsed" "$name" <<'PY'
import json,sys
path,elapsed,name=sys.argv[1],float(sys.argv[2]),sys.argv[3]
with open(path,encoding="utf-8") as f:
    data=json.load(f)
assert elapsed < 0.800, f"{name} first response exceeded 800 ms: {elapsed*1000:.1f} ms"
print(f"{name}: {elapsed*1000:.1f} ms HTTP")
PY
  cp "$file" "$BODY"
}

printf 'CENTRAL-10 Dashboard first usable data...\n'
assert_fast_json "/api/v1/dashboard/summary" "Dashboard"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
impact=d.get("impact") or {}
if impact.get("has_data") is False:
    assert impact.get("weekly_trend")==[], impact.get("weekly_trend")
    assert impact.get("trend")==[], impact.get("trend")
PY

printf 'CENTRAL-10 Partners backend read model...\n'
assert_fast_read_model "/api/v1/central/partners?limit=24&offset=0" "Partners"
partner_id="$(python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
items=d.get("items") or []
assert isinstance(d.get("kpis"),dict)
assert isinstance(d.get("pagination"),dict)
print(items[0]["id"] if items else "")
PY
)"
test -n "$partner_id"

printf 'CENTRAL-10 Partner Workspace single backend read model...\n'
assert_fast_read_model "/api/v1/central/partners/$partner_id" "Partner Workspace"
python3 - "$BODY" "$partner_id" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); pid=sys.argv[2]
assert (d.get("partner") or {}).get("id")==pid
for key in ("modules","documents","invoices","subscriptions","environments","provisioning_jobs","impact_summary","connector_credentials","portal_users","billing_events"):
    assert isinstance(d.get(key),list), (key,type(d.get(key)))
PY

printf 'CENTRAL-10 Partner Workspace module presentation read model...\n'
assert_fast_read_model "/api/v1/central/partners/$partner_id/modules?state=ALL" "Partner Workspace Modules"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
assert isinstance(d.get("items"),list)
assert isinstance(d.get("filtered_items"),list)
assert isinstance(d.get("groups"),list)
assert isinstance(d.get("kpis"),dict)
assert isinstance(d.get("active_module_keys"),list)
assert d.get("state")=="ALL"
for group in d["groups"]:
    assert isinstance(group.get("items"),list)
    assert isinstance(group.get("count"),int)
PY

printf 'CENTRAL-10 Modules + partner-grouped Commercial Matrix...\n'
assert_fast_read_model "/api/v1/central/modules?perspective=PARTNER&commercial_status=ACTIVE&commercial_limit=120" "Modules / Partner Matrix"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
registry=d.get("registry") or {}; commercial=d.get("commercial") or {}
assert isinstance(registry.get("modules"),list)
assert isinstance(registry.get("topics"),list)
assert isinstance(registry.get("kpis"),dict)
assert commercial.get("perspective")=="PARTNER"
groups=commercial.get("groups") or []
for group in groups:
    assert "partner_id" in group and "partner_name" in group
    assert isinstance(group.get("modules"),list)
PY

printf 'CENTRAL-10 module-grouped Commercial Matrix...\n'
assert_fast_read_model "/api/v1/central/modules?perspective=MODULE&commercial_status=ACTIVE&commercial_limit=120" "Module Matrix"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
commercial=d.get("commercial") or {}
assert commercial.get("perspective")=="MODULE"
groups=commercial.get("groups") or []
for group in groups:
    assert "module_key" in group and "module_label" in group
    assert isinstance(group.get("partners"),list)
PY

printf 'CENTRAL-10 Packages backend read model and canonical pricing...\n'
assert_fast_read_model "/api/v1/central/packages" "Packages"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
plans={p["plan_key"]:p for p in d.get("plans",[])}
assert plans["STARTER"]["monthly_price"]==990 and plans["STARTER"]["module_limit"]==10
assert plans["STARTER"]["display_price"]=="$990 + VAT"
assert plans["BUSINESS"]["monthly_price"]==1490 and plans["BUSINESS"]["module_limit"]==20
assert plans["BUSINESS"]["display_price"]=="$1,490 + VAT"
assert plans["FLEX"]["monthly_price"]==2490
assert plans["FLEX"]["entitlement"]=="Unlimited"
assert plans["FLEX"]["display_price"]=="$2,490 + VAT"
PY

printf 'CENTRAL-10 Finance backend read model...\n'
assert_fast_read_model "/api/v1/central/finance?invoice_status=PAID&revenue_period=WEEKLY&revenue_plan=ALL" "Finance"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
assert isinstance(d.get("overview"),dict)
assert isinstance(d.get("invoices"),list)
assert isinstance(d.get("partners"),list)
assert isinstance(d.get("onboarding"),list)
assert isinstance(d.get("kpis"),dict)
chart=d.get("chart") or {}
assert chart.get("period")=="WEEKLY", chart
assert chart.get("plan_key")=="ALL", chart
assert isinstance(chart.get("rows"),list)
assert "max_paid" in chart
for invoice in d["invoices"]:
    assert str(invoice.get("workflow_status") or invoice.get("status") or "").upper()=="PAID"
    assert "partner_name" in invoice
PY

printf 'CENTRAL-10 Impact backend read model...\n'
assert_fast_read_model "/api/v1/central/impact?evidence_limit=12&evidence_offset=0" "Impact"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
for key in ("definitions","summary","evidence","reports"):
    assert isinstance(d.get(key),list), (key,type(d.get(key)))
PY

rm -f "$TMP_ROOT"/himate-central10-*.json
echo 'CENTRAL-10 backend-first and sub-800ms runtime acceptance passed'
