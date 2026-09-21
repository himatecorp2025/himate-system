#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-profile-owner.txt"
USER_COOKIE="$TMP_ROOT/himate-profile-user.txt"
STALE_COOKIE="$TMP_ROOT/himate-profile-stale.txt"
BODY="$TMP_ROOT/himate-profile-body.json"
rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$STALE_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$STALE_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
USER_PASSWORD="$(python3 -c 'import secrets; print("Aa1!"+secrets.token_urlsafe(24))')"
USER_PASSWORD_NEW="$(python3 -c 'import secrets; print("Bb2!"+secrets.token_urlsafe(24))')"

login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'system owner login and profile... '
owner="$(login "$OWNER_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD")"
printf '%s' "$owner" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["system_owner"] is True; assert d["can_manage_users"] is True; assert d["preferred_locale"]=="en_US"'
echo ok

printf 'owner creates role-scoped employee... '
create_payload="$(python3 - "$USER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Profile Test Employee","email":"ci-profile-user@example.com","password":sys.argv[1],"roles":["operations_admin"]}))
PY
)"
created="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$create_payload" "$BASE_URL/api/v1/admin/users")"
printf '%s' "$created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["system_owner"] is False; assert d["roles"]==["operations_admin"]; assert d["preferred_locale"]=="en_US"'
echo ok

printf 'owner role cannot be delegated... '
reserved_payload="$(python3 - "$USER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Forbidden Owner","email":"ci-forbidden-owner@example.com","password":sys.argv[1],"roles":["platform_admin"]}))
PY
)"
test "$(status "$OWNER_COOKIE" POST "/api/v1/admin/users" -H 'Content-Type: application/json' -d "$reserved_payload")" = "409"
grep -q 'OWNER_ROLE_RESERVED' "$BODY"
echo ok

printf 'employee profile persists email and Hungarian preference... '
login "$USER_COOKIE" "ci-profile-user@example.com" "$USER_PASSWORD" >/dev/null
login "$STALE_COOKIE" "ci-profile-user@example.com" "$USER_PASSWORD" >/dev/null
updated="$(curl -fsS -b "$USER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"name":"Profile Test Employee","email":"ci-profile-user-renamed@example.com","preferred_locale":"hu_HU","timezone":"Europe/Budapest","job_title":"Operations Specialist","phone":"+36 1 555 0101"}' "$BASE_URL/api/v1/profile")"
printf '%s' "$updated" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["email"]=="ci-profile-user-renamed@example.com"; assert d["preferred_locale"]=="hu_HU"; assert d["timezone"]=="Europe/Budapest"; assert d["job_title"]=="Operations Specialist"; assert d["system_owner"] is False'
profile="$(curl -fsS -b "$USER_COOKIE" "$BASE_URL/api/v1/profile")"
printf '%s' "$profile" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["email"]=="ci-profile-user-renamed@example.com"; assert d["preferred_locale"]=="hu_HU"; assert d["timezone"]=="Europe/Budapest"'
echo ok

printf 'non-owner cannot manage users... '
test "$(status "$USER_COOKIE" POST "/api/v1/admin/users" -H 'Content-Type: application/json' -d "$create_payload")" = "403"
echo ok

printf 'weak password is rejected... '
weak_payload="$(python3 - "$USER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"current_password":sys.argv[1],"new_password":"onlylowercase1234"}))
PY
)"
test "$(status "$USER_COOKIE" POST "/api/v1/profile/password" -H 'Content-Type: application/json' -d "$weak_payload")" = "400"
grep -q 'lowercase, uppercase, a number and a special character' "$BODY"
echo ok

printf 'password change rotates session version... '
password_payload="$(python3 - "$USER_PASSWORD" "$USER_PASSWORD_NEW" <<'PY'
import json,sys
print(json.dumps({"current_password":sys.argv[1],"new_password":sys.argv[2]}))
PY
)"
changed="$(curl -fsS -b "$USER_COOKIE" -c "$USER_COOKIE" -H 'Content-Type: application/json' -d "$password_payload" "$BASE_URL/api/v1/profile/password")"
printf '%s' "$changed" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["preferred_locale"]=="hu_HU"'
test "$(status "$STALE_COOKIE" GET "/api/v1/auth/me")" = "401"
curl -fsS -b "$USER_COOKIE" "$BASE_URL/api/v1/auth/me" >/dev/null
echo ok

printf 'profile mutations are audited without passwords... '
sleep 1
audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/audit/events?q=profile&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); a=[x for x in d["items"] if x["action"] in ("PROFILE_UPDATED","PROFILE_PASSWORD_CHANGED")]; assert len(a)>=2,a; assert all(x["correlation_id"] for x in a)'
if printf '%s' "$audit" | grep -Fq "$USER_PASSWORD"; then exit 1; fi
if printf '%s' "$audit" | grep -Fq "$USER_PASSWORD_NEW"; then exit 1; fi
echo ok

echo "HIMATE profile, owner access and locale smoke passed"
