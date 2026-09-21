# ADR-0001: Retain a containerized microservice control plane

- **Status:** Accepted
- **Date:** 2026-09-21
- **Scope:** HIMATE control plane

## Context

The original START development blueprint recommended a modular monolith as the initial backend shape. The implemented HIMATE system evolved into independently deployable Go domain services with Docker images.

The product must support increasing partner count, operational isolation and potentially high, uneven load across domains. Reverting the existing service topology to a monolith would remove isolation that is already implemented and tested.

## Decision

HIMATE will retain the **containerized microservice architecture**.

Domain services remain separately buildable/deployable. The Gateway is the controlled browser/admin ingress; private services communicate through explicit HTTP contracts and service authentication. Partner business databases remain isolated from the control-plane database.

This decision is an intentional deviation from the original modular-monolith recommendation.

## Consequences

### Positive
- independent scaling of hot domains
- isolated rollout/failure domains
- provider/runtime operations do not require coupling to the Gateway
- clear ownership of schemas and APIs
- container-level reproducibility in CI and production
- easier future placement of queues/caches/shared rate-limit infrastructure

### Costs
- more network hops and timeout/failure modes
- distributed tracing/correlation becomes mandatory
- deployment and observability are more complex
- database connection budgets must be managed across replicas
- cross-domain transactions require explicit workflow/state-machine design

## High-load constraints

Microservices and Docker do not by themselves guarantee high-load readiness.

Before horizontally scaling multiple Gateway replicas, the current process-local login throttling must be moved to a shared/distributed store so brute-force limits are global. Durability-sensitive asynchronous work must use durable queue infrastructure when required. Load tests, connection-pool sizing, database indexes and backpressure remain production acceptance gates.

## Alternatives rejected
- **Return to modular monolith:** rejected because it would discard working isolation and independent deployment boundaries.
- **Create even more microservices for localization/profile:** rejected; these concerns belong to Identity/frontend and do not justify another network boundary.
