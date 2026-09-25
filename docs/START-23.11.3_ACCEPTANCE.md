# START-23.11.3 Acceptance — Module Marketplace

## Scope

START-23.11.3 turns the Partner Portal module surface into a discovery-first Module Marketplace without weakening module entitlement or publication controls.

The canonical HIMATE/Klavierhaus portfolio is dynamically sized and must contain at least one module. Every canonical module is discoverable by name and bilingual high-level summary even before its live implementation is released. Adding future modules must not require changing a fixed catalog-size constant.

Discovery visibility is not execution authority.

## Core contract

The Marketplace has four independent concepts:

1. **Marketplace visibility** — whether the module may be shown for product discovery.
2. **Implementation/publication readiness** — whether the module is technically released for live use.
3. **Subscription-plan entitlement** — whether the partner's current plan includes/selects the module.
4. **Operational availability** — whether the released module is currently available rather than unavailable/maintenance.

A module card may therefore be visible without being executable.

## Dynamically sized canonical module baseline

The canonical registry remains exactly:
- Finance & Invoicing: 3;
- Technical Operations: 16;
- Marketing: 8;
- Website & Events: 11.

START-23.11.3 seeds a separate bilingual marketplace summary for every canonical module. These summaries are intentionally high-level and do not claim unreconstructed implementation details.

Later reconstruction work may enrich the detailed module description without changing the stable module key or marketplace identity.

## Marketplace access states

### ACTIVE

A module is ACTIVE in the Marketplace only when:
- publication_status = PUBLISHED;
- implementation_state = READY;
- operational availability = ACTIVE;
- the partner entitlement is ACTIVE.

The Partner Portal presents it as included in the current subscription.

### LOCKED

A module is LOCKED when it is live-capable (PUBLISHED + READY + operationally ACTIVE) but is not included in the partner's current entitlement.

The card remains visible.

For plan-managed partners the Gateway enriches the card with Billing-owned plan information:
- available_in_plans;
- upgrade_plan_keys;
- recommended_upgrade_plan where applicable.

Example:
- Business partner;
- live module not included in Business;
- Flex can select the module;
- Marketplace displays it as locked and available with Flex.

A LOCKED module does not become executable merely because it is visible.

### COMING_SOON

A canonical marketplace module is COMING_SOON when it is discoverable but not both READY and PUBLISHED.

It remains visible by canonical name and marketplace summary but:
- executable = false;
- can_activate = false;
- no live module operation is exposed.

This replaces the old assumption that every unpublished canonical module must be completely invisible. Arbitrary unpublished non-marketplace modules remain hidden.

### UNAVAILABLE

A released module whose operational availability is not ACTIVE is shown as unavailable and is not executable.

## Subscription-plan authority

Billing remains the authority for Starter, Business and Flex package membership.

Catalog does not infer plan membership.

Gateway combines:
- Catalog marketplace state;
- Billing plan definitions;
- the authenticated partner's current plan.

Starter and Business use their configured fixed_module_keys.

Flex is selectable and may expose any PUBLISHED + READY + operationally ACTIVE module, subject to the existing Flex limit and entitlement rules.

Only higher-plan options are emitted as upgrade_plan_keys.

## UI behavior

Partner Portal Modules becomes **Module Marketplace**.

The page shows:
- total catalog modules;
- Included count;
- Locked count;
- Coming Soon count.

Cards are separated into:
- Included in your plan;
- Explore more modules;
- Coming soon;
- Temporarily unavailable.

Each card shows:
- canonical module name;
- high-level marketplace summary;
- module group;
- plan-access explanation;
- live availability;
- state pill.

For locked plan-managed modules with an available higher plan, the partner may navigate to **View upgrade options**. This only navigates to Billing; it does not mutate the subscription.

## Security / fail-closed rules

Marketplace visibility must never bypass:
- PUBLISHED + READY execution gates;
- partner plan entitlement;
- operational availability;
- Partner Portal permissions;
- tenant isolation.

Direct activation of a locked module for a plan-managed partner remains blocked by the existing PLAN_MANAGED_MODULES boundary.

An unpublished arbitrary module that is not marketplace_visible remains hidden, preserving START-23.11.1 behavior.

## Stabilization closure

The current START-23.11.3 baseline is the base Marketplace contract plus the active readiness/onboarding chain A and D through K.

START-23.11.3B and START-23.11.3C are historical, superseded fixed-fixture approaches. START-23.11.3D deliberately replaced them with real Partner onboarding. Their obsolete acceptance documents and executable audit/smoke scripts are therefore not part of the current source tree; the Git history remains the audit trail.

The forward migrations that remove the retired fixed fixture from long-lived databases remain required and must not be deleted.

## Automated evidence

Static audit:
- python3 scripts/audit_start_23_11_3.py

Compose acceptance:
- sh scripts/smoke_start_23_11_3.sh http://127.0.0.1:8080

The acceptance suite must prove:
1. every canonical module in the current non-empty catalog is discoverable with a non-empty marketplace summary before release;
2. discoverable unreleased modules are COMING_SOON and non-executable;
3. after publishing the canonical portfolio and configuring Business, exactly 10 canonical modules are ACTIVE for the Business acceptance partner;
4. every remaining released canonical module outside the active Business entitlement is LOCKED, not hidden; planned unreleased modules remain COMING_SOON;
5. locked canonical modules expose Flex as an upgrade path when eligible;
6. dashboard and direct Marketplace endpoints return the same enriched state;
7. direct activation cannot bypass managed-plan entitlement;
8. all prior START-23.11.1 publication/activation fail-closed behavior remains valid.

## Out of scope

START-23.11.3 does not:
- change existing canonical stable module keys;
- implement the detailed internal functionality of legacy-reference modules;
- customize module names/icons/colors per tenant — START-23.11.4 owns that;
- assign user-level module permissions — START-23.11.5 owns that;
- change plan pricing or Billing authority;
- make unpublished/incomplete modules executable.
