#!/usr/bin/env python3
from pathlib import Path
import json
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path):
    p = ROOT / path
    if not p.exists():
        raise AssertionError(f"missing required file: {path}")
    return p.read_text()

payments = read("services/cmd/payments/main.go")
billing = read("services/cmd/billing/main.go") + "\n" + read("services/cmd/billing/payment_provider.go")
gateway = read("services/cmd/gateway/main.go")
frontend = read("frontend/lib/main.dart")
compose = read("docker-compose.yml")
render = read("render.yaml")
ci = read(".github/workflows/ci.yml")
openapi = read("docs/openapi.yaml")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))

required_payments = [
    "verifyStripeSignature",
    'Header.Set("Idempotency-Key"',
    'form.Set("off_session", "true")',
    "/v1/payment_intents",
    "payments.partner_profiles",
    "payments.attempts",
    "payments.webhook_events",
    "PAYMENT_AMOUNT_MISMATCH",
    "WEBHOOK_EVENT_CONFLICT",
    "PROCESSED",
    "/internal/v1/payments/settlements",
]
for token in required_payments:
    assert token in payments, f"Payments contract missing: {token}"

for token in [
    "start234BillingPaymentMigration",
    "PROVIDER_MANAGED_PAYMENT",
    "SETTLEMENT_MISMATCH",
    "LICENSE_PAID",
    "INVOICE_PAID",
    "queueInvoiceCollection",
    "collectActivationLicense",
    "/internal/v1/payments/settlements",
]:
    assert token in billing, f"Billing payment authority missing: {token}"

assert 'mux.HandleFunc("/webhooks/stripe"' in gateway, "Gateway must expose provider webhook"
assert 'a.serveProxy(w, r, "payments")' in gateway, "Gateway must route Payments"
assert 'strings.HasSuffix(path, "/license/collect")' in gateway, "activation collection must require approval permission"

for forbidden in [
    "Verified paid amount · USD",
    "controller: paid,",
    "controller: paymentReference",
    "controller: paymentDate",
    "controller: verifiedBy",
]:
    assert forbidden not in frontend, f"manual payment UI still present: {forbidden}"
for token in [
    "/api/v1/payments/partners/",
    "/license/collect",
    "Automatic recurring collection",
    "signed provider webhook",
]:
    assert token in frontend, f"provider-backed frontend contract missing: {token}"

assert "services/docker/payments.Dockerfile" in compose
assert "PAYMENT_PROVIDER: mock" in compose
assert "PAYMENTS_HOSTPORT: payments:10000" in compose
assert "name: himate-payments" in render
assert "value: stripe" in render
assert "key: STRIPE_SECRET_KEY" in render and "key: STRIPE_WEBHOOK_SECRET" in render
assert "sync: false" in render
assert "STRIPE_SECRET_KEY:" not in compose, "real Stripe secret must not be committed to Compose"
assert "sk_live_" not in render + compose + payments
assert "whsec_" not in render, "production webhook secret must be runtime-only"

for path in [
    "/api/v1/payments/partners/{partnerId}/profile:",
    "/api/v1/payments/partners/{partnerId}/attempts:",
    "/api/v1/billing/partners/{partnerId}/license/collect:",
    "/webhooks/stripe:",
]:
    assert path in openapi, f"OpenAPI missing {path}"

contracts = {x["id"]: x for x in matrix["contracts"]}
for cid in ["PART-WIZ-LICENSE", "PART-LICENSE-EDIT", "PAYMENT-AUTOPAY"]:
    item = contracts[cid]
    assert item["target_phase"] == "23.4"
    assert item["current_state"] == "MUTATION_PROVEN_PROD_UNVERIFIED", (cid, item["current_state"])
    assert "smoke_start_23_4.sh" in item["e2e_proof"], cid

assert "audit_start_23_4.py" in ci
assert "smoke_start_23_4.sh" in ci
print("HIMATE START-23.4 provider-backed payment static audit passed")
