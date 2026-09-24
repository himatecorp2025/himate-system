# START-23.12 Phase 3B — Tenant Finance / Manual + Automated Invoice Intake Acceptance

## Scope

Phase 3B extends the Phase 3 durable automation architecture with the tenant/customer invoicing foundation. It does **not** repurpose HIMATE Platform Billing. The services/cmd/billing service remains the control-plane billing system used when HIMATE charges Partners for licenses, plans, modules and related commercial terms.

Customer-facing invoices belong to the separate tenant-finance service and the canonical Partner module invoice_documents.

## Canonical invoice sources

Every tenant/customer invoice has exactly one immutable source type:

- MANUAL — a Partner Portal user creates an invoice without Workshop Workflow.
- WORKFLOW — a future Workshop Workflow runtime emits an approved/QC-complete invoice intent.
- SCHEDULE — a future Scheduler/Calendar runtime emits an invoice intent after a simple non-workshop job is closed.

All three sources converge on the same tenant invoice domain, numbering, payment-policy snapshot, accounting-policy snapshot, durable automation event and future document/settlement lifecycle.

## Manual invoice contract — implemented

The authenticated Partner Portal runtime for invoice_documents can create and read tenant invoices and finalize a manual draft.

Tenant identity is never accepted from the manual invoice JSON body. The Gateway derives the tenant from the authenticated Partner Portal session and forwards it in the internal X-Himate-Partner-ID header. The tenant-finance service requires both Partner and user identity headers and every invoice query is filtered by partner_id.

Manual invoice creation is idempotent per (partner_id, request_key). Replaying the same request returns the original invoice. Reusing the key with different invoice content is rejected.

A MANUAL + DRAFT invoice may be replaced through the own-tenant draft update route before final review. The update recalculates all line/tax totals server-side inside one SQL transaction and replaces the draft items under the same tenant key. Once the invoice leaves DRAFT, silent draft edits are rejected.

Money is stored in integer minor units. Fractional quantities use quantity_milli, and tax uses basis points. The service calculates and persists immutable line totals instead of trusting a browser-calculated invoice total.

## Final financial review boundary

A manual invoice starts as DRAFT.

Finalization changes it to READY_FOR_ISSUE and atomically freezes:

- the current Partner issuer/company identity from the authoritative Partners service,
- Partner logo URL,
- current Partner Finance policy,
- invoice-level payment-terms override, when present,
- accounting basis,
- allowed payment methods,
- customer billing snapshot,
- calculated invoice lines and totals,
- tenant invoice-number prefix/policy snapshot for the later issuance transaction.

The Partner company data are read at finalization time, not copied from the browser. Later Corporate Data changes therefore cannot rewrite an already finalized invoice snapshot.

Configured payment terms remain policy-driven. There is no hardcoded eight-day invoice rule. The deployed platform default is explicitly configured through HIMATE_TENANT_FINANCE_DEFAULT_TERMS_DAYS and can be overridden by each Partner and by an individual invoice.

## Durable automation

The transition to READY_FOR_ISSUE and the corresponding tenant_invoice.ready_for_issue.v1 automation intent are committed in the same SQL transaction through the Phase 3 transactional outbox.

The tenant-finance outbox publisher signs events with the dedicated finance automation service identity and sends them to the durable Automation service. Delivery therefore survives retries/restarts and cannot silently lose the finance continuation after the invoice transaction commits.

## Workflow and Calendar/Scheduler intake — prepared, producer runtimes deferred

Automated invoice intake uses the durable Phase 3 Automation bus; there is no direct Workshop/Scheduler-to-Finance command endpoint.

Tenant Finance registers itself as consumer service identity finance for two canonical event contracts:

- workflow.billing_approved.v1
- scheduler.job_closed_invoice_ready.v1

The future Workshop service publishes the workflow event under its own HMAC identity and the future Scheduler publishes the schedule event under its own identity. Automation verifies the producer signature, persists the immutable event, and creates a leased/retriable delivery for Finance.

Finance receives only its dedicated HIMATE_AUTOMATION_FINANCE_SECRET. It does not receive the Workshop or Scheduler HMAC secrets.

When Finance claims a delivery it derives authoritative routing identity from the immutable event envelope:

- WORKFLOW requires producer_service=workshop, module_key=workshop_workflow, subject_type=workflow.
- SCHEDULE requires producer_service=scheduler, module_key=scheduler, subject_type=scheduled_job.
- partner_id comes from the event envelope.
- source_id comes from event subject_id.

The event payload contains only invoice business data (customer, lines, currency, optional payment-terms override and notes). Unknown fields are rejected, so the producer payload cannot override tenant/source identity.

Automated sources are idempotent under (partner_id, source_type, source_id). A retry returns the same invoice; a second different invoice for the same source is rejected.

This creates the production-grade target integration paths:

Workshop QC/admin approval -> signed workflow.billing_approved.v1 -> durable Automation delivery -> WORKFLOW invoice -> Tenant Finance

and

Calendar/Scheduler simple job closed -> signed scheduler.job_closed_invoice_ready.v1 -> durable Automation delivery -> SCHEDULE invoice -> Tenant Finance.

The actual Workshop and Calendar/Scheduler business runtimes remain outside this Phase 3B change and must publish these contracts when those modules are built.

## Legal document boundary

READY_FOR_ISSUE is deliberately **not** equivalent to ISSUED.

Phase 3B does not claim that a legally compliant invoice PDF/document has already been rendered. The ISSUED state is reserved for the future jurisdiction-aware document renderer/delivery workflow. READY_FOR_ISSUE deliberately does not consume an official invoice number; it snapshots the tenant numbering prefix/policy, and the later ISSUED transaction must allocate the legal invoice number atomically with document issuance. That renderer must use the frozen issuer/customer/line/policy snapshot and must satisfy the applicable invoice-document requirements before transitioning the invoice to ISSUED.

This prevents a backend record from being presented as a legally issued invoice before the official document engine exists.

## Tenant isolation and authorization

Partner Portal access is the intersection of:

1. organization ownership/entitlement of invoice_documents,
2. authenticated user module assignment,
3. role permission (billing.read for reads; billing.write for mutations),
4. authenticated Partner identity carried server-to-server.

An invoice identifier by itself never authorizes access. Direct reads use the authenticated partner_id together with the invoice id.

## Acceptance proof

Static architecture:
- scripts/audit_start_23_12_phase3b.py

Runtime/container proof:
- scripts/smoke_start_23_12_phase3b.sh

Unit proof:
- services/cmd/tenantfinance/domain_test.go

The runtime smoke proves:
- manual draft creation,
- same-request idempotent replay,
- conflicting replay rejection,
- manual DRAFT edit and server-side recalculation,
- post-finalization edit rejection,
- client-supplied partner_id rejection,
- issuer data refreshed from authoritative Partner master data at finalization,
- tenant cross-read rejection,
- invoice policy snapshot and configurable terms,
- signed SCHEDULE producer event -> durable Automation delivery -> Finance invoice,
- signed WORKFLOW producer event -> durable Automation delivery -> Finance invoice,
- producer/event/module/subject impersonation rejection,
- producer payload cannot override envelope tenant/source identity,
- automated-source idempotency,
- Finance receives only its dedicated automation secret,
- durable publication of resulting finance events back to the Automation backbone.

## Closure marker

MANUAL_TENANT_INVOICE_BACKEND = IMPLEMENTED

WORKFLOW_INVOICE_INTAKE_CONTRACT = READY / PRODUCER_RUNTIME_DEFERRED

SCHEDULE_INVOICE_INTAKE_CONTRACT = READY / PRODUCER_RUNTIME_DEFERRED

LEGAL_INVOICE_DOCUMENT_RENDERER = DEFERRED

PLATFORM_BILLING_SEPARATION = PRESERVED
