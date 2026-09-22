# START-23.2 Acceptance — Partner × Module Commercial Control Plane & Individual Pricing

## Purpose

START-23.2 closes the partner-by-partner module commercial-control gap identified by START-23.1.

The administrator must be able to answer, from one authoritative control plane:

- which partner has which module;
- whether that assignment is active, not licensed or under maintenance;
- whether the module is visible to the partner;
- whether it is included in the base service;
- the module default 30-day recurring price;
- the partner-specific 30-day recurring price;
- the module default one-time activation fee;
- the partner-specific one-time activation fee;
- the module activation date;
- the current immutable 30-day subscription period and price;
- the next billing date and exact next-period price;
- whether renewal/cancellation is pending;
- the commercial change history.

## Explicit boundary

START-23.2 does **not** finish the cancellation state machine or payment-provider collection.

- START-23.3 will make Billing the authoritative lifecycle command for period-end cancellation and automatic expiry.
- START-23.4 will implement real payment-provider charging, webhook settlement and collection.

START-23.2 may display the existing cancellation state, but it must not claim that the current admin entitlement mutation is the final lifecycle implementation.

## Data model

Catalog remains authoritative for module registry, partner assignment and configured commercial terms.

START-23.2 adds:

- module-level `default_activation_fee`;
- partner-level `activation_fee_override`;
- immutable/effective-dated activation-fee history;
- current and future effective recurring-price metadata;
- current and future effective activation-fee metadata;
- normalized partner-module commercial history.

Existing recurring-price history remains effective-dated and auditable.

Billing remains authoritative for immutable paid-period snapshots and subscriptions.

## Partner × Module matrix

The Modules control plane must provide a central commercial matrix with:

- partner and module search;
- partner filter;
- module filter;
- entitlement-state filter;
- explicit **View by partner** and **View by module** perspectives;
- current period price;
- current period boundaries;
- next billing date;
- exact next-period price quoted at the next period boundary;
- configured recurring price and its source;
- scheduled next configured recurring price;
- activation fee and source;
- partner visibility;
- renewal/cancellation status;
- commercial history;
- partner-specific commercial edit.

The matrix must remain responsive and must not use a raw desktop-only DataTable.

## Exact next-period quote

The next billing price must not be guessed in Flutter.

Billing batches the current subscription boundaries and asks Catalog for point-in-time price quotes at each exact next-period start. Catalog resolves the same effective-dated pricing history used by invoice generation.

The UI consumes Billing's `next_period_price`, `next_period_included_in_base`, `next_period_currency` and `next_billing_date`.

## Duplicate-path removal

The legacy module create/edit controls embedded in the Licensing & Finance page are removed.

Module registry mutations belong to the Modules control plane.

This eliminates duplicate UI ownership while preserving the Billing page for issuer/company/commercial-finance concerns.

## Required E2E proof

`scripts/smoke_start_23_2.sh` must prove at least:

1. module group creation;
2. two module creations with default recurring and activation fees;
3. module default commercial update and readback;
4. relationship create/read/delete;
5. partner commercial terms and agreement readback;
6. partner module activation/assignment;
7. partner-specific recurring price and activation-fee override;
8. matrix readback;
9. commercial history readback;
10. Billing subscription creation;
11. exact next billing date/price;
12. future price and activation-fee scheduling without mutating the current period;
13. next-period quote adopts the scheduled price;
14. gateway audit observes the commercial mutation.

## Definition of Done

START-23.2 is done only when:

- Go tidy/vet/unit/race/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23.1 contract audit remains green;
- START-23.2 static contract audit passes;
- the full START-01–23.1 regression remains green;
- START-23.2 mutation smoke passes in Compose;
- the 23.2 branch is merged to `develop` with no unique commits left outside the merge path.

START-24 remains blocked.
