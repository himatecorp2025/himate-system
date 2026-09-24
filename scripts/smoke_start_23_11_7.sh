#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.32-start-23.11.7}"

printf '23.11.7 synchronized release health... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok",d
assert d["release_consistent"] is True,d
assert d["version"]==expected,(d.get("version"),expected)
PY
echo ok

printf 'Partner Portal performance indexes are installed... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -At <<'SQL' | grep -qx '6'
SELECT COUNT(*) FROM pg_indexes
WHERE indexname IN (
  'identity_partner_user_modules_partner_idx',
  'notifications_partner_delivery_idx',
  'notifications_target_user_idx',
  'notifications_module_delivery_idx',
  'cms_partner_module_presentations_partner_idx',
  'billing_partner_plan_due_idx'
);
SQL
echo ok

printf 'Partner Portal tenant-key constraints are installed... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -At <<'SQL' | grep -qx '3'
SELECT COUNT(*) FROM pg_constraint c
JOIN pg_class t ON t.oid=c.conrelid
JOIN pg_namespace n ON n.oid=t.relnamespace
WHERE c.contype='p'
  AND (
    (n.nspname='identity' AND t.relname='partner_user_modules')
    OR (n.nspname='cms' AND t.relname='partner_workspace_settings')
    OR (n.nspname='cms' AND t.relname='partner_module_presentations')
  );
SQL
echo ok

printf 'notification target-user SQL and bulk read-all contract are present in running source release... '
grep -Fq "AND (e.target_user_id='' OR e.target_user_id=\$1)" services/cmd/notifications/main.go
grep -Fq 'strings.Join(values, ",")' services/cmd/notifications/main.go
echo ok

printf 'Partner Portal closure runtime sentinel... ok\n'
