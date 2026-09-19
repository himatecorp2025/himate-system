# START-04–08 architecture

Browser -> HIMATE Gateway / Flutter
              |
              +-- Partner Service
              +-- Catalog Service
              +-- Billing Service
              |
              +-- HIMATE PostgreSQL

Each backend domain is an independently deployable Go binary and Docker image. Internal services are private and require a service-to-service credential. They own separate PostgreSQL schemas today, preserving a clean migration path to physically separate databases later. Partner business databases are never merged into HIMATE.
