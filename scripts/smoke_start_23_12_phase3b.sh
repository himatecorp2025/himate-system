#!/bin/sh
set -eu

BASE="${1:-http://127.0.0.1:18083}"
AUTO_BASE="${2:-http://127.0.0.1:18082}"
VERSION="${HIMATE_APP_VERSION:-0.8.33-start-23.12}"

docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO partners.partners(
  id,slug,display_name,legal_name,lifecycle,country,state_region,city,postal_code,address_line1,logo_url,
  finance_contact_email,existing_partner,reference_partner,test_partner
) VALUES
  ('ptr_phase3b_a','phase3b-a','Phase3B A','Phase3B A LLC Draft','LIVE','United States','NY','New York','10001','10 Finance Ave','https://example.test/a-logo.png','finance-a@example.test',TRUE,FALSE,TRUE),
  ('ptr_phase3b_b','phase3b-b','Phase3B B','Phase3B B LLC','LIVE','United States','NY','New York','10002','20 Finance Ave','https://example.test/b-logo.png','finance-b@example.test',TRUE,FALSE,TRUE)
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
  finance_contact_email=EXCLUDED.finance_contact_email,
  test_partner=TRUE;
SQL

python3 - "$BASE" "$AUTO_BASE" "$VERSION" <<'PY'
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
auto_base = sys.argv[2].rstrip("/")
version = sys.argv[3]
token = "local-development-internal-token-123456789"
partner_a = "ptr_phase3b_a"
partner_b = "ptr_phase3b_b"

def http(base_url, method, path, payload=None, partner=None, user=None, headers=None, want=(200,)):
    raw = b""
    if payload is not None:
        raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    req = urllib.request.Request(base_url + path, data=raw if payload is not None else None, method=method)
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

def signed_event(service, secret, envelope, want=(200, 201)):
    path = "/internal/v1/automation/events"
    raw = json.dumps(envelope, separators=(",", ":"), sort_keys=True).encode()
    ts = str(int(time.time()))
    digest = hashlib.sha256(raw).hexdigest()
    canonical = "\n".join([ts, service, "POST", path, digest]).encode()
    signature = "sha256=" + hmac.new(secret.encode(), canonical, hashlib.sha256).hexdigest()
    return http(auto_base, "POST", path, envelope, headers={
        "X-Himate-Service-ID": service,
        "X-Himate-Service-Timestamp": ts,
        "X-Himate-Service-Signature": signature,
        "X-Himate-Correlation-ID": "phase3b-correlation",
    }, want=want)

_, health, _ = http(base, "GET", "/health")
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
_, saved_policy, _ = http(base, "PUT", "/internal/v1/tenant-finance/policy", policy, partner_a, "pusr_phase3b_admin")
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
    "items": [{
        "description": "Piano tuning",
        "quantity_milli": 1000,
        "unit_price_minor": 25000,
        "discount_minor": 0,
        "tax_rate_bps": 887,
    }],
    "payment_terms_days": 15,
    "notes": "Simple job outside Workshop Workflow",
}
_, draft, _ = http(base, "POST", "/internal/v1/tenant-finance/invoices", manual, partner_a, "pusr_phase3b_admin", want=(201,))
assert draft["source_type"] == "MANUAL" and draft["status"] == "DRAFT", draft
assert draft["partner_id"] == partner_a and draft["total_minor"] == 27218, draft
invoice_id = draft["id"]

_, replay, _ = http(base, "POST", "/internal/v1/tenant-finance/invoices", manual, partner_a, "pusr_phase3b_admin", want=(200,))
assert replay["duplicate"] is True and replay["id"] == invoice_id, replay

changed = json.loads(json.dumps(manual))
changed["items"][0]["unit_price_minor"] = 26000
_, conflict, _ = http(base, "POST", "/internal/v1/tenant-finance/invoices", changed, partner_a, "pusr_phase3b_admin", want=(409,))
assert conflict["error"]["code"] == "REQUEST_KEY_CONFLICT", conflict

malicious = json.loads(json.dumps(manual))
malicious["request_key"] = "phase3b-malicious-tenant"
malicious["partner_id"] = partner_b
_, rejected, _ = http(base, "POST", "/internal/v1/tenant-finance/invoices", malicious, partner_a, "pusr_phase3b_admin", want=(400,))
assert rejected["error"]["code"] == "JSON", rejected

draft_update = {
    "currency": "USD",
    "customer": manual["customer"],
    "items": [{
        "description": "Piano tuning - reviewed",
        "quantity_milli": 1000,
        "unit_price_minor": 25500,
        "discount_minor": 0,
        "tax_rate_bps": 887,
    }],
    "payment_terms_days": 16,
    "notes": "Reviewed simple job outside Workshop Workflow",
}
_, updated, _ = http(base, "PUT", f"/internal/v1/tenant-finance/invoices/{invoice_id}", draft_update, partner_a, "pusr_phase3b_admin")
assert updated["status"] == "DRAFT" and updated["total_minor"] == 27762, updated
assert updated["payment_terms_override_days"] == 16, updated
assert updated["items"][0]["description"] == "Piano tuning - reviewed", updated

print(invoice_id)
PY

docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<'SQL'
UPDATE partners.partners
SET legal_name='Phase3B A LLC Final', updated_at=NOW()
WHERE id='ptr_phase3b_a';
SQL

python3 - "$BASE" "$AUTO_BASE" "$VERSION" <<'PY'
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
auto_base = sys.argv[2].rstrip("/")
version = sys.argv[3]
token = "local-development-internal-token-123456789"
partner_a = "ptr_phase3b_a"
partner_b = "ptr_phase3b_b"

def http(base_url, method, path, payload=None, partner=None, user=None, headers=None, want=(200,)):
    raw = b""
    if payload is not None:
        raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    req = urllib.request.Request(base_url + path, data=raw if payload is not None else None, method=method)
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

def publish(service, secret, envelope, want=(200, 201)):
    path = "/internal/v1/automation/events"
    raw = json.dumps(envelope, separators=(",", ":"), sort_keys=True).encode()
    ts = str(int(time.time()))
    digest = hashlib.sha256(raw).hexdigest()
    canonical = "\n".join([ts, service, "POST", path, digest]).encode()
    sig = "sha256=" + hmac.new(secret.encode(), canonical, hashlib.sha256).hexdigest()
    req = urllib.request.Request(auto_base + path, data=raw, method="POST")
    req.add_header("X-Himate-Internal-Token", token)
    req.add_header("X-Himate-Expected-Version", version)
    req.add_header("Content-Type", "application/json")
    req.add_header("X-Himate-Service-ID", service)
    req.add_header("X-Himate-Service-Timestamp", ts)
    req.add_header("X-Himate-Service-Signature", sig)
    req.add_header("X-Himate-Correlation-ID", "phase3b-correlation")
    try:
        with urllib.request.urlopen(req, timeout=8) as resp:
            body = resp.read()
            status = resp.status
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read()
    if status not in want:
        raise AssertionError(f"publish {service}: status {status}, wanted {want}, body={body.decode(errors='replace')}")
    return status, json.loads(body or b"{}")

_, listing = http(base, "GET", "/internal/v1/tenant-finance/invoices?limit=50", partner=partner_a, user="pusr_phase3b_admin")
manual = next(x for x in listing["items"] if x["request_key"] == "phase3b-manual-001")
manual_id = manual["id"]

_, ready = http(base, "POST", f"/internal/v1/tenant-finance/invoices/{manual_id}/finalize", {}, partner_a, "pusr_phase3b_admin")
assert ready["status"] == "READY_FOR_ISSUE", ready
assert ready["source_type"] == "MANUAL", ready
assert ready["payment_terms_days"] == 16 and ready["accounting_basis"] == "ACCRUAL", ready
assert ready["invoice_number"] == "", ready
assert ready["invoice_prefix_snapshot"] == "P3A", ready
assert ready["issuer"]["partner_id"] == partner_a, ready
assert ready["issuer"]["legal_name"] == "Phase3B A LLC Final", ready
assert ready["issuer"]["logo_url"] == "https://example.test/a-logo.png", ready
assert ready["document_state"]["renderer"] == "DEFERRED", ready
prefix_snapshot = ready["invoice_prefix_snapshot"]

frozen_update = {
    "currency": "USD",
    "customer": ready["customer"],
    "items": [{
        "description": "Forbidden post-finalization edit",
        "quantity_milli": 1000,
        "unit_price_minor": 1,
        "discount_minor": 0,
        "tax_rate_bps": 0,
    }],
}
_, frozen = http(base, "PUT", f"/internal/v1/tenant-finance/invoices/{manual_id}", frozen_update, partner_a, "pusr_phase3b_admin", want=(409,))
assert frozen["error"]["code"] == "INVOICE_STATE", frozen

_, replay = http(base, "POST", f"/internal/v1/tenant-finance/invoices/{manual_id}/finalize", {}, partner_a, "pusr_phase3b_admin")
assert replay["duplicate"] is True and replay["invoice_number"] == "" and replay["invoice_prefix_snapshot"] == prefix_snapshot, replay

_, cross = http(base, "GET", f"/internal/v1/tenant-finance/invoices/{manual_id}", partner=partner_b, user="pusr_phase3b_billing", want=(404,))
assert cross["error"]["code"] == "INVOICE_NOT_FOUND", cross

invoice_payload = {
    "currency": "USD",
    "customer": {
        "display_name": "Automated Customer",
        "country": "United States",
        "state_region": "NY",
        "city": "New York",
        "postal_code": "10004",
        "address_line1": "40 Calendar St",
    },
    "items": [{
        "description": "Simple automated tuning",
        "quantity_milli": 1000,
        "unit_price_minor": 18000,
        "discount_minor": 0,
        "tax_rate_bps": 0,
    }],
}

schedule_event = {
    "event_key": "phase3b-schedule-001",
    "event_type": "scheduler.job_closed_invoice_ready.v1",
    "partner_id": partner_a,
    "module_key": "scheduler",
    "subject_type": "scheduled_job",
    "subject_id": "calendar-simple-job-001",
    "payload": invoice_payload,
}
status, scheduled_event = publish("scheduler", "scheduler-automation-secret-local-123456789", schedule_event, want=(201,))
assert scheduled_event["duplicate"] is False, scheduled_event
status, schedule_replay = publish("scheduler", "scheduler-automation-secret-local-123456789", schedule_event, want=(200,))
assert schedule_replay["duplicate"] is True and schedule_replay["id"] == scheduled_event["id"], schedule_replay

workflow_event = {
    "event_key": "phase3b-workflow-001",
    "event_type": "workflow.billing_approved.v1",
    "partner_id": partner_a,
    "module_key": "workshop_workflow",
    "subject_type": "workflow",
    "subject_id": "workflow-qc-approved-001",
    "payload": invoice_payload,
}
publish("workshop", "workshop-automation-secret-local-123456789", workflow_event, want=(201,))

# A scheduler cannot impersonate Workshop merely by choosing the Workshop event type.
bad_event = {
    "event_key": "phase3b-impersonation-001",
    "event_type": "workflow.billing_approved.v1",
    "partner_id": partner_a,
    "module_key": "workshop_workflow",
    "subject_type": "workflow",
    "subject_id": "impersonation-attempt",
    "payload": invoice_payload,
}
publish("scheduler", "scheduler-automation-secret-local-123456789", bad_event, want=(201,))

deadline = time.time() + 25
scheduled = flowed = None
while time.time() < deadline:
    _, listing = http(base, "GET", "/internal/v1/tenant-finance/invoices?limit=100", partner=partner_a, user="pusr_phase3b_admin")
    for item in listing["items"]:
        if item["source_type"] == "SCHEDULE" and item["source_id"] == "calendar-simple-job-001":
            scheduled = item
        if item["source_type"] == "WORKFLOW" and item["source_id"] == "workflow-qc-approved-001":
            flowed = item
    if scheduled and flowed:
        break
    time.sleep(1)

assert scheduled is not None and scheduled["status"] == "READY_FOR_ISSUE", scheduled
assert flowed is not None and flowed["status"] == "READY_FOR_ISSUE", flowed
assert scheduled["partner_id"] == partner_a and flowed["partner_id"] == partner_a
assert scheduled["issuer"]["legal_name"] == "Phase3B A LLC Final"
assert flowed["issuer"]["legal_name"] == "Phase3B A LLC Final"

_, listing = http(base, "GET", "/internal/v1/tenant-finance/invoices?limit=100", partner=partner_a, user="pusr_phase3b_admin")
assert not any(x["source_id"] == "impersonation-attempt" for x in listing["items"]), listing

print("START-23.12 Phase 3B tenant invoicing runtime smoke: manual + durable bus assertions PASS")
PY

i=0
while [ "$i" -lt 25 ]; do
  count="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COUNT(*) FROM automation.events WHERE producer_service='finance' AND partner_id='ptr_phase3b_a' AND event_type='tenant_invoice.ready_for_issue.v1'")"
  failed_impersonation="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COUNT(*) FROM automation.deliveries d JOIN automation.events e ON e.id=d.event_id WHERE d.consumer_service='finance' AND e.event_key='phase3b-impersonation-001' AND d.last_error LIKE '%producer must be workshop%'")"
  if [ "$count" -ge 3 ] && [ "$failed_impersonation" -ge 1 ]; then
    echo "START-23.12 Phase 3B durable finance events PASS ($count events); producer impersonation rejected by Finance consumer"
    exit 0
  fi
  i=$((i+1))
  sleep 1
done

docker compose exec -T postgres psql -U himate -d himate -c "SELECT e.producer_service,e.event_key,e.event_type,e.partner_id,d.consumer_service,d.status,d.last_error FROM automation.events e LEFT JOIN automation.deliveries d ON d.event_id=e.id WHERE e.event_key LIKE 'phase3b-%' OR e.producer_service='finance' ORDER BY e.id DESC LIMIT 30"
echo "Expected durable finance output events and a rejected producer-impersonation delivery"
exit 1
