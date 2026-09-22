#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start233-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-start233-partner.txt"
BODY="$TMP_ROOT/himate-start233-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
GROUP_KEY="ci_233_$STAMP"
MODULE_KEY="ci.start233_$STAMP"
PORTAL_EMAIL="start233-$STAMP@example.com"
PORTAL_PASSWORD="Strong-Start233!$STAMP"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.3 owner login... '
payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create lifecycle partner and module... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.3 Lifecycle Partner","legal_name":"START 23.3 Lifecycle Partner LLC","brand_name":"START 23.3","contact_name":"Lifecycle Owner","contact_email":"lifecycle@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$(python3 - "$GROUP_KEY" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label":"START 23.3 Lifecycle","sort_order":233}))
PY
)" "$BASE_URL/api/v1/module-groups" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$(python3 - "$MODULE_KEY" "$GROUP_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],"label":"START 23.3 Lifecycle Module","group_key":sys.argv[2],
 "description":"Authoritative period-end lifecycle acceptance","currency":"USD",
 "version":"1.0.0","latest_version":"1.0.0","default_monthly_price":33,
 "default_activation_fee":0,"availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE",
 "owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)" "$BASE_URL/api/v1/modules" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"status":"ACTIVE","visible":true,"included_in_base":false,"partner_price":33,"reason":"START-23.3 activation"}' "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/summary" >/dev/null
subs="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$subs" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["lifecycle_state"]=="ACTIVE",s; assert s["auto_renew"] is True,s; assert s["cancel_at_period_end"] is False,s' "$MODULE_KEY"
period_end="$(printf '%s' "$subs" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; print(next(x for x in d["items"] if x["module_key"]==key)["period_end_exclusive"])' "$MODULE_KEY")"
period_start="$(printf '%s' "$subs" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; print(next(x for x in d["items"] if x["module_key"]==key)["period_start"])' "$MODULE_KEY")"
echo ok

printf 'direct admin entitlement bypass is rejected... '
test "$(status "$OWNER_COOKIE" PATCH "/api/v1/partners/$partner_id/modules/$MODULE_KEY" -H 'Content-Type: application/json' -d '{"status":"NOT_LICENSED","reason":"bypass attempt"}')" = "409"
grep -q 'BILLING_LIFECYCLE_REQUIRED' "$BODY"
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules")"
printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; assert next(x for x in d["items"] if x["key"]==key)["status"]=="ACTIVE"' "$MODULE_KEY"
echo ok

printf 'maintenance cannot be used as a deactivation bypass... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"status":"MAINTENANCE","reason":"START-23.3 maintenance lifecycle test"}' "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null
test "$(status "$OWNER_COOKIE" PATCH "/api/v1/partners/$partner_id/modules/$MODULE_KEY" -H 'Content-Type: application/json' -d '{"status":"NOT_LICENSED","reason":"maintenance bypass attempt"}')" = "409"
grep -q 'BILLING_LIFECYCLE_REQUIRED' "$BODY"
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules")"
printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; assert next(x for x in d["items"] if x["key"]==key)["status"]=="MAINTENANCE"' "$MODULE_KEY"
echo ok

printf 'admin schedules period-end cancellation in Billing... '
cancelled="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":true,"reason":"START-23.3 admin cancellation"}' "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions/$MODULE_KEY")"
printf '%s' "$cancelled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="CANCEL_PENDING",d; assert d["cancel_at_period_end"] is True,d; assert d["auto_renew"] is False,d; assert d["cancellation_effective_at"]==sys.argv[1],d' "$period_end"
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules")"
printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; assert next(x for x in d["items"] if x["key"]==key)["status"]=="MAINTENANCE"' "$MODULE_KEY"
echo ok

printf 'Partner Portal uses the same Billing state machine to withdraw and reschedule while Catalog is in maintenance... '
portal_payload="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.3 Portal Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$partner_id/portal-users" >/dev/null
login_payload="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
withdrawn="$(curl -fsS -b "$PARTNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":false}' "$BASE_URL/partner/api/v1/modules/$MODULE_KEY/subscription")"
printf '%s' "$withdrawn" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="ACTIVE",d; assert d["cancel_at_period_end"] is False,d; assert d["auto_renew"] is True,d'
rescheduled="$(curl -fsS -b "$PARTNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":true}' "$BASE_URL/partner/api/v1/modules/$MODULE_KEY/subscription")"
printf '%s' "$rescheduled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="CANCEL_PENDING",d; assert d["cancellation_effective_at"]==sys.argv[1],d' "$period_end"
echo ok

curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"status":"ACTIVE","reason":"Restore service before scheduled paid-period expiry"}' "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null

printf 'exact period-end Billing cycle expires entitlement without renewal... '
docker compose exec -T billing /app/service --run-invoice-cycle "$period_end"
subs_after="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$subs_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); key,start,end=sys.argv[1:]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["lifecycle_state"]=="INACTIVE",s; assert s["payment_status"]=="INACTIVE",s; assert s["period_start"]==start,s; assert s["period_end_exclusive"]==end,s; assert s["auto_renew"] is False,s' "$MODULE_KEY" "$period_start" "$period_end"
catalog_after="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules")"
printf '%s' "$catalog_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; assert next(x for x in d["items"] if x["key"]==key)["status"]=="NOT_LICENSED"' "$MODULE_KEY"
echo ok

printf 'period-end cycle is idempotent and history is retained... '
docker compose exec -T billing /app/service --run-invoice-cycle "$period_end"
subs_repeat="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$subs_repeat" | python3 -c 'import json,sys; d=json.load(sys.stdin); key,start,end=sys.argv[1:]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["lifecycle_state"]=="INACTIVE",s; assert s["period_start"]==start,s; assert s["period_end_exclusive"]==end,s' "$MODULE_KEY" "$period_start" "$period_end"
history_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.subscription_history WHERE partner_id='$partner_id' AND module_key='$MODULE_KEY' AND new_lifecycle_state IN ('ACTIVE','CANCEL_PENDING','INACTIVE');")"
test "$history_count" -ge 4
events="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/events")"
printf '%s' "$events" | python3 -c 'import json,sys; d=json.load(sys.stdin); types={x["event_type"] for x in d["items"]}; required={"MODULE_CANCELLATION_SCHEDULED","MODULE_CANCELLATION_WITHDRAWN","MODULE_CANCELLATION_EFFECTIVE","MODULE_PERIOD_ENDED"}; assert required <= types,(required-types,d)'
echo ok

echo "HIMATE START-23.3 authoritative cancellation state-machine smoke passed"
