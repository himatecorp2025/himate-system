# START-20 Acceptance — Domains and provider deployments

## Domain controls
- [x] STAGING and PRODUCTION records are separate.
- [x] DNS verification performs a real resolver lookup.
- [x] TLS verification performs a real certificate/TLS connection check.
- [x] Production launch blocks on DNS/TLS/runtime/deployment gates.
- [x] LIVE hostname changes are locked.

## Deployment provider controls
- [x] Provider mechanics are isolated in the Runtime microservice.
- [x] Runtime has an adapter interface.
- [x] Local adapter provides deterministic CI behavior.
- [x] Render adapter calls the real Render Deploy API.
- [x] Render API credential is runtime-secret configuration.
- [x] Commit ID and clear-build-cache options are supported.
- [x] Provider deployment ID/status/error are persisted.
- [x] Provider deployment states normalize to DEPLOYING/READY/FAILED.
- [x] Environment is not marked DEPLOYED until provider READY.
- [x] Runtime health polling finalizes asynchronous deployments.
- [x] Provider adapter has an httptest-backed unit test.
- [x] Docker Compose smoke proves local provider persistence.
- [x] Render Blueprint declares the Render provider and secret inputs.

## Production note
CI deliberately does **not** execute a real Render deployment. The adapter is tested against an HTTP contract test server, while production credentials/service IDs are injected at runtime. A production deployment is an operational action, not a unit/integration-test side effect.

## Evidence
- `services/cmd/runtime/main.go`
- `services/cmd/runtime/main_test.go`
- `services/cmd/environments/main.go`
- `docker-compose.yml`
- `render.yaml`
- `scripts/smoke_start_20.sh`

START-20 is complete only when Go tests, Compose topology, provider persistence and START-20 smoke are green.
