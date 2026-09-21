# ADR-0005: Klavierhaus one-way data connector and unified seven-year retention

- Status: Accepted
- Date: 2026-09-21
- Decision scope: START-22

## Context

Klavierhaus currently runs a Node.js/Express/SQLite ERP while HIMATE uses independently deployable Go microservices, PostgreSQL and a Flutter administration frontend. HIMATE needs approved operational/business measurements from Klavierhaus without coupling either system to the other's implementation language or database.

Direct database access would create a brittle cross-system dependency, bypass partner isolation and make data-minimization controls difficult to prove. Copying raw Klavierhaus tables would also unnecessarily replicate customer, employee, credential and payment data.

## Decision

START-22 uses an API-contract boundary:

```text
Klavierhaus Node.js ERP
        |
        | privacy-safe collectors
        v
Klavierhaus HIMATE Export Adapter
        |
        | HTTPS JSON + bearer identity
        | HMAC-SHA-512 + nonce + timestamp
        v
HIMATE Gateway -> Connector Go microservice
        |
        +--> retained allowlisted Connector record
        +--> numeric KPI -> Impact
        +--> sync/reconciliation -> System Health
```

The primary data direction is Klavierhaus -> HIMATE. HIMATE does not receive SQL credentials for the Klavierhaus database.

The contract is language-neutral. A future Klavierhaus Go backend must implement the same Connector Protocol v1 rather than requiring a HIMATE redesign.

## Mapping boundary

Exactly 38 Klavierhaus functional modules are represented by a machine-readable registry. Each registry entry declares:
- module_key
- dataset_key
- transfer_mode
- target_service
- cadence
- personal/sensitive-data classification
- explicit allowed_fields
- schema_version

Unknown datasets and fields fail closed. Collectors query aggregates and approved company metadata rather than serializing raw rows.

Secrets, passwords, sessions, API/OAuth credentials, invitation/preview tokens and raw payment credentials are not part of the export contract.

## Security

Partner/environment identity is derived from the bearer credential, never from request JSON.

Signed START-22 requests include a timestamp, credential-scoped nonce, SHA-512 body digest and HMAC-SHA-512 signature. HIMATE enforces a five-minute timestamp window, constant-time signature comparison and nonce replay protection.

Payloads are bounded to 1 MiB and 250 items. Data values are bounded scalar values under the field allowlist.

Accepted Connector business payloads are encrypted at rest using AES-256-GCM envelope encryption. Each retained record receives a random 256-bit data-encryption key and independent nonce. The record key is itself wrapped with a versioned runtime-only AES-256 master key; only ciphertext, nonces, wrapped key material and key version are stored. The active master key is supplied as a deployment secret and previous key versions may be retained in the service keyring for controlled rotation. Plaintext payload JSON is not retained in PostgreSQL.

## Retention

All accepted START-22 Connector data uses the HIMATE product policy `HIMATE_7Y`:
- retain_until defaults to receipt time plus seven years;
- routed Impact observations inherit the same policy and deadline;
- legal hold blocks deletion in both Connector and Impact;
- mandatory privacy deletion propagates to Impact before the Connector record is purged;
- expired records are removed by service-local retention workers.

Seven years is an internal HIMATE retention policy, not a statement that every US record category has a universal seven-year statutory minimum. Mandatory legal/privacy deletion requirements and legal holds override the default policy.

## Consequences

Positive:
- Node.js and future Go Klavierhaus backends remain interchangeable behind the contract.
- HIMATE never needs Klavierhaus database credentials.
- Data collection is reviewable at field level.
- Retention, provenance and reconciliation are explicit.
- Existing Impact/Reports/System Health capabilities are reused instead of duplicated.

Trade-offs:
- Aggregation intentionally removes some raw analytical flexibility.
- Schema/registry changes require coordinated protocol evolution.
- Reconciliation and retention add durable Connector state.
- Legal/privacy overrides must be synchronized across Connector and routed downstream copies.

## Rejected alternatives

### Direct HIMATE SQL access to Klavierhaus
Rejected because it couples databases, weakens isolation and bypasses the export/privacy boundary.

### Raw 38-module database replication
Rejected because it would unnecessarily duplicate personal/sensitive data and make retention harder to govern.

### Rewrite Klavierhaus to Go before integration
Rejected because protocol-level integration does not require a shared implementation language and would delay the business data contract.

### Per-record-type retention in START-22
Deferred. START-22 deliberately uses one seven-year HIMATE policy for accepted data, with legal hold and mandatory deletion overrides.
