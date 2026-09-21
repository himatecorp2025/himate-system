# ADR-0003: Provider-neutral deployment adapter in the Runtime service

- **Status:** Accepted
- **Date:** 2026-09-21
- **Scope:** START-20 deployments

## Context

The previous Runtime service persisted a deployment row and immediately returned READY. It did not invoke a hosting provider, so the Environments state machine could report DEPLOYED without a real deployment.

The architecture must remain provider-neutral and containerized.

## Decision

The existing private `runtime` microservice becomes the deployment-provider boundary.

It exposes the existing internal Runtime contract to Environments and selects an adapter:
- `local` for deterministic Docker Compose/CI
- `render` for production Render deployments

The Render adapter uses the Render Deploy API, Bearer authentication, an optional commit ID and optional clear-build-cache flag. Provider deployment ID/status/error are persisted.

The Environments service treats provider acceptance as `DEPLOYING`. `active_release` and `DEPLOYED` are committed only when Runtime reports provider state `READY`.

## Secret handling

`RENDER_API_KEY` is a runtime secret and is never committed. A Render service ID may come from environment-specific deployment config or an optional runtime default.

## Consequences

- hosting details stay out of Gateway and business lifecycle code
- provider integration can scale/fail independently
- CI never triggers production infrastructure
- additional providers can implement the same adapter interface
- asynchronous provider states become first-class and must be polled/observed
