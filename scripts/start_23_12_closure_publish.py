#!/usr/bin/env python3
import base64
import hashlib
import hmac
import json
import os
import sys
import time
import urllib.request

if len(sys.argv) != 7:
    raise SystemExit("usage: publisher BASE PARTNER STAMP CALLER EVENT_TYPE SUBJECT_ID")

base, partner, stamp, caller, event_type, subject_id = sys.argv[1:]
token = os.environ["CLOSURE_INTERNAL_TOKEN"]
secret = os.environ["CLOSURE_AUTOMATION_SECRET"]
path = "/internal/v1/automation/events"

if event_type == "workflow.billing_approved.v1":
    module_key = "workshop_workflow"
    subject_type = "workflow"
elif event_type == "scheduler.job_closed_invoice_ready.v1":
    module_key = "scheduler"
    subject_type = "scheduled_job"
else:
    raise SystemExit("unsupported closure event type")

payload = {
    "event_key": f"closure-{caller}-{subject_id}-{stamp}",
    "event_type": event_type,
    "partner_id": partner,
    "module_key": module_key,
    "subject_type": subject_type,
    "subject_id": f"{subject_id}-{stamp}",
    "payload": {
        "currency": "USD",
        "customer": {
            "display_name": "Closure Customer",
            "company_name": "Closure Customer LLC",
            "country": "United States",
            "state_region": "NY",
            "city": "New York",
            "postal_code": "10001",
            "address_line1": "1 Closure Way",
            "email": "closure-customer@example.test",
        },
        "items": [{
            "description": "START-23.12 cross-phase service",
            "quantity_milli": 1000,
            "unit_price_minor": 15000,
            "discount_minor": 0,
            "tax_rate_bps": 0,
        }],
        "payment_terms_days": 10,
        "notes": "START-23.12 cross-phase production closure",
    },
}
raw = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
timestamp = str(int(time.time()))

body_hash = hashlib.sha256(raw).hexdigest()
automation_canonical = "\n".join([timestamp, caller, "POST", path, body_hash]).encode()
automation_signature = "sha256=" + hmac.new(
    secret.encode(), automation_canonical, hashlib.sha256
).hexdigest()

caller_key = hmac.new(
    token.encode(), ("himate-service-v1|" + caller).encode(), hashlib.sha256
).digest()
service_canonical = "\n".join(
    ["POST", path, "", caller, timestamp, "", "", "", "", ""]
).encode()
service_signature = base64.urlsafe_b64encode(
    hmac.new(caller_key, service_canonical, hashlib.sha256).digest()
).decode().rstrip("=")

request = urllib.request.Request(base.rstrip("/") + path, data=raw, method="POST")
for key, value in {
    "Content-Type": "application/json",
    "X-Himate-Internal-Token": token,
    "X-Himate-Caller-ID": caller,
    "X-Himate-Caller-Timestamp": timestamp,
    "X-Himate-Caller-Signature": service_signature,
    "X-Himate-Service-ID": caller,
    "X-Himate-Service-Timestamp": timestamp,
    "X-Himate-Service-Signature": automation_signature,
}.items():
    request.add_header(key, value)

with urllib.request.urlopen(request, timeout=10) as response:
    if response.status != 201:
        raise SystemExit(f"unexpected publish status {response.status}")
    sys.stdout.write(response.read().decode())
