# START-23.10 Acceptance — System & Operations Production Closure

## Scope

START-23.10 closes the System & Operations contracts identified by START-23.1. It does **not** begin START-23.11 Partner Portal Commercial Parity.

The phase covers:

- new-partner Provisioning Engine preparation, execution and idempotent retry;
- connector credential one-time disclosure, authenticated use and rotation invalidation;
- Partner Website Adapter domain binding without rewriting the partner website;
- environment create/edit/provider actions and deployment-state persistence;
- production runtime-provider fail-closed behavior;
- partner-scoped backup-policy persistence and deterministic scheduler proof;
- preservation of the already production-proven encrypted backup and restore-verification contracts;
- EN/HU completion for the directly affected operations controls.

## Production provider boundary

Runtime owns deployment-provider integration.

- Local/CI uses the deterministic `local` provider.
- Render production uses `HIMATE_RUNTIME_PROVIDER=render`, runtime-only `RENDER_API_KEY`, and runtime-only `HIMATE_RENDER_SERVICE_ID`.
- A production process configured for a real provider rejects an environment-level attempt to downgrade the deployment to `local`.
- No provider secret is committed.
- Actual live Render/DNS/TLS evidence remains a START-23.12 production-acceptance proof. START-23.10 proves the production adapter, configuration boundary, persistence and fail-closed semantics.

## Provisioning contract

A commercially cleared partner in `READY_TO_PROVISION` can be prepared through `POST /api/v1/provisioning/jobs` with `prepare_only=true`, then executed through `POST /api/v1/provisioning/jobs/{jobId}/run`.

The engine must persist its step state and produce isolated partner infrastructure. A second run after `CONFIGURATION_REQUIRED` is idempotent: succeeded steps are not repeated and their attempt counts do not increase.

## Connector credential contract

`POST /api/v1/connectors/{partnerId}/credential` returns the raw credential exactly once.

- only the token hash is persisted;
- an issued token authenticates the credential-bound connector API;
- rotation replaces the active hash for that partner/environment;
- the old raw token immediately fails authentication;
- the new raw token succeeds;
- administrative GET responses never expose either raw token or token hash.

## Website Adapter contract

The Website Adapter configures how the partner's **existing** website talks to HIMATE; the website itself is not replaced or reprogrammed.

Credential-bound commercial state fails closed when the request host is outside the partner's allowed domains, and succeeds only after the adapter is bound to the partner's configured domain.

## Environment contract

Environment create/edit/provider operations persist authoritative state in Environments while Runtime owns the deployment provider.

CI proves the lifecycle with the deterministic local provider. Production configuration proves Render is the default real provider and cannot be silently overridden to local.

DNS/TLS verification failures are persisted and block launch. Positive live external DNS/TLS evidence remains START-23.12.

## Backup policy and scheduler contract

Backup policy remains partner-scoped.

`PUT /api/v1/backups/policies/{partnerId}` persists retention, maximum restore points, schedule interval and enabled state.

The same scheduling logic used by the background scheduler is exposed to authorized operations through `POST /api/v1/backups/scheduler/run`, allowing deterministic operational execution and acceptance proof. Policy reads expose `last_scheduled_at`.

The existing START-21 production evidence for encrypted durable backup creation and restore verification remains authoritative and is not downgraded.

## Functional matrix closure

There are exactly 11 START-23.10 contracts.

- `BACKUP-CREATE` and `BACKUP-RESTORE-TEST` remain `PROD_PROVEN`.
- the remaining nine contracts become `MUTATION_PROVEN_PROD_UNVERIFIED`.
- all 11 are localization-complete for their directly affected UI surface.
- live-provider proof remains explicitly deferred to START-23.12.

## Required quality gates

- Go tidy, vet, unit, race and build;
- Flutter analyze, browser tests and release build;
- all historical START-23.1 through START-23.9 audits;
- START-23.10 static audit;
- full historical Compose regression;
- START-23.10 mutation smoke;
- exact-head PR CI green before merge;
- merge-commit `develop` CI green after merge.

## Release

`0.8.14-start-23.10`

## Phase boundary

START-23.11 must not begin automatically. Partner Portal Commercial Parity is intentionally a separate phase and requires product-scope discussion before implementation.
