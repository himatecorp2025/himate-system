# START-01–08 corrected architecture

```text
Browser
  |
  v
HIMATE Gateway / Identity / Flutter + Public Website
  |
  +-- private Partner Service
  +-- private Catalog Service
  +-- private Billing Service
  +-- private Contact Service
  |
  +-- HIMATE PostgreSQL
       +-- identity schema
       +-- partners schema
       +-- catalog schema
       +-- billing schema
       +-- contact schema
```

Each backend domain is an independently deployable Go binary and Docker image. Only the Gateway is exposed to the browser. Internal services require the shared service credential and are reached through private service addresses. Runtime images are distroless and run as non-root.

The services currently share one HIMATE PostgreSQL instance but own separate schemas. Database changes are service-scoped, ordered, transactional migrations recorded in a shared migration registry. Registry creation is serialized before individual service migration locks are acquired, so parallel container startup does not race on bootstrap DDL. The boundaries preserve a direct path to physically separated service databases later. Partner business databases are never merged into HIMATE.

Partner Portfolio reads are paginated at the Partner service. The Gateway requests only the matching page's IDs from Catalog and Billing, then aggregates those bounded results in parallel. Dashboard summaries use bounded queries and a short-lived cache.

The Billing service uses activation-date anchored 30-day cycles. A daily containerized cron checks which partners are exactly at a cycle boundary; it does not treat calendar month boundaries as subscription boundaries. Effective-dated partner-module pricing and initial-license evidence remain auditable.

The public Contact endpoint is proxied by the Gateway to the private Contact service. Inquiries are stored in the `contact` schema and can optionally trigger SMTP notification delivery when configured.

Production topology is declared in `render.yaml`; local and CI integration testing use the same gateway/partners/catalog/billing/contact boundaries through `docker-compose.yml`.
