#!/bin/sh
set -eu

BASE="${1:-http://127.0.0.1:18083}"
VERSION="${HIMATE_APP_VERSION:-0.8.32-start-23.11.7}"

docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO partners.partners(
  id,slug,display_name,legal_name,lifecycle,country,state_region,city,postal_code,address_line1,logo_url,
  finance_contact_email,existing_partner,reference_partner
) VALUES
  ('ptr_phase3b_a','phase3b-a','Phase3B A','Phase3B A LLC Draft','LIVE','United States','NY','New York','10001','10 Finance Ave','https://example.test/a-logo.png','finance-a@example.test',TRUE,FALSE),
  ('ptr_phase3b_b','phase3b-b','Phase3B B','Phase3B B LLC','LIVE','United States','NY','New York','10002','20 Finance Ave','https://example.test/b-logo.png','finance-b@example.test',TRUE,FALSE)
ON CONFLICT(id) DO UPDATE SET
  display_name=EXCLUDED.display_name,
  legal_name=EXCLUDED.legal_name,
  lifecycle='LIVE',
  country=EXCLUDED.country,
  state_region=EXCLUDED.state_region,
  city=EXCLUDED.city,
  postal_code=EXCLUDED.postal_code,
  address_line1=EXCLUDED.address_line1,
  logo_url=EXCLUDED.logo_url,
  finance_contact_email=EXCLUDED.finance_contact_email;
SQL

python3 - "$BASE" "$VERSION" <<'PY'
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
version = sys.argv[2]
token = "local-development-internal-token-123456789"
partner_a = "ptr_phase3b_a"
partner_b = "ptr_phase3b_b"

def http(method, path, payload=None, partner=None, user=None, headers=None, want=(200,)):
    raw = b""
    if payload is not None:
        raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    req = urllib.request.Request(base + path, data=raw if payload is not None else None, method=method)
    req.add_header("X-Himate-Internal-Token", token)
    req.add_header("X-Himate-Expected-Version", version)
    if payload is not None:
        req.add_header("Content-Type", "application/json")
    if partner:
        req.add_header("X-Himate-Partner-ID", partner)
    if user:
        req.add_header("X-Himate-User-ID", user)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=8) as resp:
            body = resp.read()
            status = resp.status
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read()
    if status not in want:
        raise AssertionError(f"{method} {path}: status {status}, wanted {want}, body={body.decode(errors='replace')}")
    return status, json.loads(body or b"{}")

status, health = http("GET", "/health", want=(200,))
assert health["service"] == "tenant-finance", health
assert health["document_renderer"] == "DEFERRED", health
assert set(health["invoice_sources"]) == {"MANUAL", "WORKFLOW", "SCHEDULE"}, health

policy = {
    "default_payment_terms_days": 12,
    "accounting_basis": "ACCRUAL",
    "payment_methods": ["ZELLE", "CHECK"],
    "default_currency": "USD",
    "invoice_prefix": "P3A",
}
_, saved_policy = http("PUT", "/internal/v1/tenant-finance/policy", policy, partner_a, "pusr_phase3b_admin")
assert saved_policy["policy"]["default_payment_terms_days"] == 12, saved_policy
assert saved_policy["policy"]["accounting_basis"] == "ACCRUAL", saved_policy
assert saved_policy["invoice_prefix"] == "P3A", saved_policy

manual = {
    "request_key": "phase3b-manual-001",
    "currency": "USD",
    "customer": {
        "display_name": "Manual Customer",
        "company_name": "Manual Customer LLC",
        "country": "United States",
        "state_region": "NY",
        "city": "New York",
        "postal_code": "10003",
        "address_line1": "30 Customer St",
        "email": "billing@customer.example",
    },
    "items": [
        {
            "description": "Piano tuning",
            "quantity_milli": 1000,
            "unit_price_minor": 25000,
            "discount_minor": 0,
            "tax_rate_bps": 887,
        }
    ],
    "payment_terms_days": 15,
    "notes": "Simple job outside Workshop Workflow",
}
status, draft = http("POST", "/internal/v1/tenant-finance/invoices", manual, partner_a, "pusr_phase3b_admin", want=(201,))
assert draft["source_type"] == "MANUAL", draft
assert draft["status"] == "DRAFT", draft
assert draft["partner_id"] == partner_a, draft
assert draft["total_minor"] == 27218, draft
invoice_id = draft["id"]

status, replay = http("POST", "/internal/v1/tenant-finance/invoices", manual, partner_a, "pusr_phase3b_admin", want=(200,))
assert replay["duplicate"] is True and replay["id"] == invoice_id, replay

changed = json.loads(json.dumps(manual))
changed["items"][0]["unit_price_minor"] = 26000
status, conflict = http("POST", "/internal/v1/tenant-finance/invoices", changed, partner_a, "pusr_phase3b_admin", want=(409,))
assert conflict["error"]["code"] == "REQUEST_KEY_CONFLICT", conflict

malicious = json.loads(json.dumps(manual))
malicious["request_key"] = "phase3b-malicious-tenant"
malicious["partner_id"] = partner_b
status, rejected = http("POST", "/internal/v1/tenant-finance/invoices", malicious, partner_a, "pusr_phase3b_admin", want=(400,))
assert rejected["error"]["code"] == "JSON", rejected

# The issuer snapshot must be read at finalization time from the authoritative Partners service.
# Change the legal name after draft creation to prove that the final snapshot is current.
PY

docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<'SQL'
UPDATE partners.partners
SET legal_name='Phase3B A LLC Final', updated_at=NOW()
WHERE id='ptr_phase3b_a';
SQL

python3 - "$BASE" "$VERSION" <<'PY'
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
version = sys.argv[2]
token = "local-development-internal-token-123456789"
partner_a = "ptr_phase3b_a"
partner_b = "ptr_phase3b_b"
manual_invoice_id = None

def http(method, path, payload=None, partner=None, user=None, headers=None, want=(200,)):
    raw = b""
    if payload is not None:
        raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    req = urllib.request.Request(base + path, data=raw if payload is not None else None, method=method)
    req.add_header("X-Himate-Internal-Token", token)
    req.add_header("X-Himate-Expected-Version", version)
    if payload is not None:
        req.add_header("Content-Type", "application/json")
    if partner:
        req.add_header("X-Himate-Partner-ID", partner)
    if user:
        req.add_header("X-Himate-User-ID", user)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=8) as resp:
            body = resp.read()
            status = resp.status
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read()
    if status not in want:
        raise AssertionError(f"{method} {path}: status {status}, wanted {want}, body={body.decode(errors='replace')}")
    return status, json.loads(body or b"{}"), raw

_, listing, _ = http("GET", "/internal/v1/tenant-finance/invoices?limit=50", partner=partner_a, user="pusr_phase3b_admin")
manual = next(x for x in listing["items"] if x["request_key"] == "phase3b-manual-001")
manual_invoice_id = manual["id"]

_, ready, _ = http("POST", f"/internal/v1/tenant-finance/invoices/{manual_invoice_id}/finalize", {}, partner_a, "pusr_phase3b_admin")
assert ready["status"] == "READY_FOR_ISSUE", ready
assert ready["source_type"] == "MANUAL", ready
assert ready["payment_terms_days"] == 15, ready
assert ready["accounting_basis"] == "ACCRUAL", ready
assert ready["invoice_number"].startswith("P3A-"), ready
assert ready["issuer"]["partner_id"] == partner_a, ready
assert ready["issuer"]["legal_name"] == "Phase3B A LLC Final", ready
assert ready["issuer"]["logo_url"] == "https://example.test/a-logo.png", ready
assert ready["document_state"]["renderer"] == "DEFERRED", ready
number = ready["invoice_number"]

_, replay, _ = http("POST", f"/internal/v1/tenant-finance/invoices/{manual_invoice_id}/finalize", {}, partner_a, "pusr_phase3b_admin")
assert replay["duplicate"] is True and replay["invoice_number"] == number, replay

status, cross, _ = http("GET", f"/internal/v1/tenant-finance/invoices/{manual_invoice_id}", partner=partner_b, user="pusr_phase3b_billing", want=(404,))
assert cross["error"]["code"] == "INVOICE_NOT_FOUND", cross

def signed_intent(service, secret, payload, want=(200, 201)):
    path = "/internal/v1/tenant-finance/automation/invoice-intents"
    raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    ts = str(int(time.time()))
    digest = hashlib.sha256(raw).hexdigest()
    canonical = "\n".join([ts, service, "POST", path, digest]).encode()
    signature = "sha256=" + hmac.new(secret.encode(), canonical, hashlib.sha256).hexdigest()
    headers = {
        "X-Himate-Service-ID": service,
        "X-Himate-Service-Timestamp": ts,
        "X-Himate-Service-Signature": signature,
        "X-Himate-Correlation-ID": "phase3b-correlation",
    }
    req = urllib.request.Request(base + path, data=raw, method="POST")
    req.add_header("X-Himate-Internal-Token", token)
    req.add_header("X-Himate-Expected-Version", version)
    req.add_header("Content-Type", "application/json")
    for k, v in headers.items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=8) as resp:
            body = resp.read()
            status = resp.status
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read()
    if status not in want:
        raise AssertionError(f"signed intent {service}: status {status}, wanted {want}, body={body.decode(errors='replace')}")
    return status, json.loads(body or b"{}")

common = {
    "partner_id": partner_a,
    "currency": "USD",
    "customer": {
        "display_name": "Scheduled Customer",
        "country": "United States",
        "state_region": "NY",
        "city": "New York",
        "postal_code": "10004",
        "address_line1": "40 Calendar St",
    },
    "items": [{
        "description": "Simple calendar tuning",
        "quantity_milli": 1000,
        "unit_price_minor": 18000,
        "discount_minor": 0,
        "tax_rate_bps": 0,
    }],
}
schedule = dict(common)
schedule.update({"source_type": "SCHEDULE", "source_id": "calendar-simple-job-001"})
status, scheduled = signed_intent("scheduler", "scheduler-automation-secret-local-123456789", schedule, want=(201,))
assert scheduled["source_type"] == "SCHEDULE" and scheduled["status"] == "READY_FOR_ISSUE", scheduled
status, scheduled_replay = signed_intent("scheduler", "scheduler-automation-secret-local-123456789", schedule, want=(200,))
assert scheduled_replay["duplicate"] is True and scheduled_replay["id"] == scheduled["id"], scheduled_replay

workflow = dict(common)
workflow.update({"source_type": "WORKFLOW", "source_id": "workflow-qc-approved-001"})
status, flowed = signed_intent("workshop", "workshop-automation-secret-local-123456789", workflow, want=(201,))
assert flowed["source_type"] == "WORKFLOW" and flowed["status"] == "READY_FOR_ISSUE", flowed

bad = dict(common)
bad.update({"source_type": "WORKFLOW", "source_id": "impersonation-attempt"})
status, denied = signed_intent("scheduler", "scheduler-automation-secret-local-123456789", bad, want=(403,))
assert denied["error"]["code"] == "SOURCE_SERVICE_MISMATCH", denied

print("START-23.12 Phase 3B tenant invoicing runtime smoke: API assertions PASS")
PY

# The manual + SCHEDULE + WORKFLOW transitions must be published through the durable Phase 3 outbox.
i=0
while [ "$i" -lt 20 ]; do
  count="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COUNT(*) FROM automation.events WHERE producer_service='finance' AND partner_id='ptr_phase3b_a' AND event_type='tenant_invoice.ready_for_issue.v1'")"
  if [ "$count" -ge 3 ]; then
    echo "START-23.12 Phase 3B tenant invoicing runtime smoke: durable automation PASS ($count events)"
    exit 0
  fi
  i=$((i+1))
  sleep 1
done

docker compose exec -T postgres psql -U himate -d himate -c "SELECT producer_service,event_key,event_type,partner_id,module_key FROM automation.events WHERE producer_service='finance' ORDER BY id DESC LIMIT 20"
echo "Expected at least three durable tenant invoice events"
exit 1
