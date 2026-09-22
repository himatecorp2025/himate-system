# START-23.11.1 Acceptance — Module Registry & Individual Commercial Model

## Scope

START-23.11.1 establishes the authoritative data model for the future Partner Workspace without implementing START-23.11.2 billing-cycle semantics or later Partner Portal product surfaces.

The phase closes when the HIMATE control plane has one canonical module registry, distinct platform publication and partner entitlement states, and partner-specific contractual pricing that cannot be confused with module reference/list prices.

## Canonical module registry

The Klavierhaus legacy system is represented by **38 real reference modules**, not placeholders. Their current implementations remain legacy references and are not falsely represented as already rewritten in Flutter/Go.

Primary navigation grouping is fixed for the canonical set:

- Finance & Invoicing — 3 modules
- Technical Operations — 16 modules
- Marketing — 8 modules
- Website & Events — 11 modules

Workshop Workflow belongs to Technical Operations.

## Lifecycle model

Platform lifecycle and tenant entitlement are independent:

- publication status: `UNPUBLISHED | PUBLISHED`
- implementation state: `LEGACY_REFERENCE | IN_DEVELOPMENT | READY`
- partner entitlement: `INACTIVE | ACTIVE | CANCEL_PENDING`
- operational availability remains independently represented by the existing availability/maintenance controls.

A module cannot become `PUBLISHED` unless its implementation state is `READY`. Partner Portal reads and activation fail closed for `UNPUBLISHED` modules.

The 38 Klavierhaus modules are seeded as `LEGACY_REFERENCE` and `UNPUBLISHED` until explicitly brought through the new release lifecycle. This keeps the registry truthful while preserving the legacy implementation as the functional reference for later Flutter/Go rebuilds.

## Individual commercial terms

The authoritative charging model is **partner contract / quote**, never the catalog reference price.

Partner-level terms persist:

- negotiated one-time license/activation fee
- negotiated base monthly service fee
- minimum monthly commitment
- quote/offer reference
- contract currency
- pricing model `INDIVIDUAL_QUOTE`
- effective date / service anchor
- commercial configuration state
- monotonic terms version and readback history

For USD contracts, the current HIMATE minimum monthly commitment is **USD 1,500**. The old hard-coded USD 13,000 minimum activation fee is removed; activation/license fees are negotiated partner-by-partner and only negative values are invalid.

Partner-module commercial configuration persists independently for every partner:

- included-in-base flag
- negotiated recurring module fee
- negotiated module activation fee
- contract currency
- quote/offer reference
- effective-dated price/activation-fee history
- explicit `commercial_configured` marker

Catalog-level default prices remain reference values only. API responses identify `PARTNER_CONTRACT` as the pricing authority and label catalog fallback values as reference-only. Billing-facing price resolution never falls back to a catalog reference price: an add-on must have an effective partner price or be explicitly included in the base service. Partner Portal exposes this derived state as `commercial_ready` and fails closed when it is false.

## Concurrency and audit

Commercial-term updates are versioned and serialized against concurrent writes. Every accepted terms mutation stores an immutable historical snapshot with actor and reason. Partner-module price and activation-fee histories retain the contract currency and quote reference that authorized the change.

## Compatibility

Existing START-23.2 commercial-control-plane and START-23.3 cancellation state-machine behavior remains authoritative. Billing remains the only authority for final period-end deactivation. Legacy `status` fields remain supported while the new entitlement state is kept synchronized.

Historical acceptance fixtures that exercise Partner Portal lifecycle use explicit `READY + PUBLISHED` test modules.

## Explicitly not in START-23.11.1

This phase does **not** implement:

- calendar-month invoice calculation or the newly agreed full-period/no-proration rule (START-23.11.2)
- Partner Module Marketplace/self-service product UX (START-23.11.3)
- one default landing module per partner (START-23.11.4)
- employee-by-module permissions (START-23.11.5)
- workflow/calendar comment notifications (START-23.11.6)
- final Partner Portal closure audit (START-23.11.7)
- rebuilding the 38 legacy modules in Flutter/Go

## Automated evidence

Static:
- `python3 scripts/audit_start_23_11_1.py`

Containerized mutation/readback:
- `sh scripts/smoke_start_23_11_1.sh http://127.0.0.1:8080`

The smoke proves:
1. exact 38-module canonical registry and 3/16/8/11 grouping,
2. four primary navigation groups,
3. truthful legacy-reference state,
4. fail-closed publish-before-ready behavior,
5. USD 1,500 minimum commitment,
6. negotiated activation fee below the obsolete USD 13,000 floor,
7. versioned commercial history,
8. two partners receiving different recurring and activation prices for the same module,
9. unpublished-module Partner Portal invisibility and activation rejection,
10. READY → PUBLISHED exposure without replacing partner-specific contractual pricing,
11. published modules without partner-specific commercial configuration fail closed,
12. Partner Portal activation writes ACTIVE entitlement and Billing cancellation synchronizes CANCEL_PENDING/ACTIVE back to Catalog without overriding operational maintenance state,
13. Billing-facing price resolution rejects unconfigured partner modules instead of using catalog reference pricing.

START-23.11.2 must not begin automatically after this phase; it remains a separate development and acceptance boundary.
