# START-23.4 Acceptance — Provider-Backed Activation & Recurring Payments

## Purpose

START-23.4 closes the payment-provider gap left by START-23.1 and START-23.3. HIMATE must no longer accept an administrator-entered paid amount, payment date, payment reference or verifier as evidence that money was collected.

Billing remains the financial source of truth for activation-license and invoice state. The Payments microservice owns provider profiles, charge attempts, provider identifiers, signed webhook verification and provider reconciliation.

## Service boundary

```text
Admin / New Partner Wizard
        |
        | commercial amount + provider profile
        v
Gateway -> Billing ------------------------------+
        |                                       |
        | activation-license / invoice intent   |
        v                                       |
Payments microservice                            |
        |                                       |
        | provider API (Stripe in production)   |
        v                                       |
Payment provider                                 |
        |                                       |
        | signed webhook                        |
        v                                       |
Gateway /webhooks/stripe -> Payments             |
        |                                       |
        | verified internal settlement          |
        +-------------------------------> Billing
                                                 |
                                                 +--> PAID / FAILED
                                                 +--> immutable billing events
```

The provider can never write Billing tables directly. Payments can only request a settlement through Billing's private internal endpoint after provider evidence is verified.

## Provider contract

Production uses the Payments adapter in `stripe` mode.

PaymentIntent creation must:
- use the saved provider customer and payment-method identifiers;
- set `confirm=true`;
- set `off_session=true` for server-side recurring collection;
- send an HTTP `Idempotency-Key`;
- persist the HIMATE attempt before the provider request;
- retain provider PaymentIntent ID and current attempt status.

CI/development uses the deterministic `mock` provider while exercising the same persisted attempt, signed-webhook and Billing-settlement flow.

Provider secrets are runtime-only configuration. They must not be committed to source control.

## Webhook authority

`POST /webhooks/stripe` is intentionally unauthenticated by HIMATE session cookies because Stripe cannot use a HIMATE user session. It must fail closed unless:
- the signature is valid for the exact raw request body;
- the signature timestamp is inside the configured tolerance;
- the webhook PaymentIntent matches the persisted attempt;
- successful amount and currency match the persisted attempt;
- the provider event ID is not reused with different payload bytes.

A webhook remains retryable until Billing settlement succeeds. Only then is the provider event marked `PROCESSED`.

Repeated delivery of the exact same already-processed provider event is idempotent.

## Initial activation license

Admin may configure:
- currency;
- required activation-license amount;
- waiver state/reason;
- note.

Admin may not directly write:
- `paid_amount`;
- `payment_date`;
- `payment_reference`;
- `verified_by`.

Attempting that returns `PROVIDER_MANAGED_PAYMENT`.

The activation-license collection command is:

`POST /api/v1/billing/partners/{partnerId}/license/collect`

It requires Billing approval permission and creates an idempotent provider attempt.

The activation license becomes `PAID` only after a verified successful provider webhook is settled by Billing. Billing then stores the provider payment ID as the payment reference and `payments-service` as verifier.

## Recurring invoice autopay

Each 30-day invoice is immutable before collection. Billing asks Payments to collect that exact invoice total using idempotency key `invoice:{invoiceId}`.

A partner must have:
- provider customer ID;
- saved payment method ID;
- autopay enabled.

A missing/unready payment profile must not abort the entire daily Billing scheduler. The invoice remains unpaid with collection pending and may be retried later.

A successful signed webhook changes the exact invoice to `PAID` and records `INVOICE_PAID`. A failed webhook records provider failure evidence without falsifying payment success.

## Required proof

`scripts/smoke_start_23_4.sh` must prove:
1. manual activation-license PAID transition is rejected;
2. provider profile persists;
3. activation charge creation is idempotent;
4. invalid webhook signatures fail closed;
5. signed but amount-mismatched webhook fails closed;
6. correctly signed provider success is the only path to `LICENSE_PAID`;
7. provisioning commercial gate opens after provider-backed PAID state;
8. daily Billing cycle creates the recurring invoice;
9. automatic collection creates a persisted invoice payment attempt;
10. signed invoice webhook changes that invoice to PAID;
11. duplicate provider delivery is idempotent;
12. payment attempt and immutable Billing event evidence remain queryable.

## Definition of Done

START-23.4 is complete only when:
- Go tidy/vet/unit/race/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23.1–23.3 regression gates remain green;
- START-23.4 static audit passes;
- START-23.4 provider/payment mutation smoke passes in Docker Compose;
- Render declares the isolated Payments service and its secret/runtime bindings;
- Stripe secrets are absent from source control;
- the feature branch is merged to `develop` with no unique commits left outside the merge path.

Real production-provider proof remains reserved for the final production verification phase. START-24 remains blocked until START-23.5–23.12 close the remaining matrix blockers.
