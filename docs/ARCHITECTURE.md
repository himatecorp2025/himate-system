# START-04–08 architecture + stabilization services

```text
Browser
  |
  v
HIMATE Gateway / Identity / Flutter + Public Website
  |
  +-- Partner Service
  +-- Catalog Service
  +-- Billing Service
  +-- Contact Service
  |
  +-- HIMATE PostgreSQL
```

Each backend domain is an independently deployable Go binary and Docker image. Internal services are private and require a service-to-service credential. They own separate PostgreSQL schemas today, preserving a clean migration path to physically separate databases later. Partner business databases are never merged into HIMATE.

The public Contact endpoint is proxied by the gateway to the private Contact Service. Inquiries are persisted in the `contact` schema and can optionally trigger SMTP notification delivery when the corresponding environment variables are configured.

Production is defined in `render.yaml`; local development uses the same gateway/partners/catalog/billing/contact service topology through `docker-compose.yml`.
