# CENTRAL-12 — Performance Recovery & Runtime Corrections

## Scope

CENTRAL-12 is one vertical performance/reliability release. It does not redesign Modules or Packages.

### Critical paths

- Partners default first page is served from a persisted materialized hot snapshot.
- Filtered/search/paginated Partners views retain the authoritative live read-model path.
- Snapshot responses preserve the caller's Catalog, Billing and Health permissions.
- Partner/catalog/billing mutations queue the relevant snapshot refresh.
- Licensing & Finance retains the materialized finance snapshot and replaces unbounded 400 ms polling with bounded warm-up retries.
- Login prewarm is staged so first-screen requests are not competing with every preset/status/system read at once.

### Analytics and exports

- Package Analytics remains backed by the Billing analytics endpoint and the persisted package-analytics snapshot.
- Partners, Packages, Finance and Impact bulk PDF exports expose an availability probe.
- The Flutter UI probes availability before download and shows an explicit "No exportable data" modal instead of downloading an empty PDF.

## Acceptance

Static:

```
python3 scripts/audit_central_12.py
```

Containerized runtime:

```
sh scripts/smoke_central_12.sh http://127.0.0.1:8080
```

The runtime gate verifies:

1. Gateway and private service health.
2. Partners default view becomes a `hot-snapshot` response.
3. Hot Partners and Finance reads stay below the existing 800 ms first-usable-data budget.
4. Finance materialization reaches `ready=true`.
5. Package Analytics returns authoritative runtime structure.
6. Empty Partners, Finance and Impact export probes return `has_data=false`.
7. Package PDF availability returns an explicit boolean/count instead of an empty PDF contract.

CENTRAL-12 is mergeable only when Go, Flutter, Compose and the full existing containerized acceptance chain remain green.
