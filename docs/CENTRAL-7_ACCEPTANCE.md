# CENTRAL-7 — Central Manual Audit Part 1 Regression Closure

CENTRAL-7 is the regression-closure phase for the first half of the Central manual audit. It adds no new product capability. Its purpose is to prove that CENTRAL-1 through CENTRAL-6 remain correct together after all first-half changes have landed.

## Authoritative scope

The first Central half is:

1. CENTRAL-1 — Landing, Contact, Partner MFA policy
2. CENTRAL-2 — Dashboard
3. CENTRAL-3 — Partners, parent navigation, bilingual coverage
4. CENTRAL-4 — Module Registry, topic/category architecture, dynamic cardinality and usage
5. CENTRAL-5 — Packages, module assignment, pricing/VAT and Unlimited entitlement
6. CENTRAL-6 — Licensing & Finance, onboarding, payment and invoice lifecycle
7. CENTRAL-7 — full regression audit of CENTRAL-1 through CENTRAL-6

CENTRAL-7 must not introduce a seventh product domain. Any product defect found by the closure is fixed in the owning CENTRAL-1..6 contract and then re-proven by CENTRAL-7.

## Static acceptance

scripts/audit_central_7.py must prove all of the following in one aggregated preflight:

- every CENTRAL-1..6 static acceptance exists and remains wired into CI in deterministic order;
- CENTRAL-7 static acceptance runs after CENTRAL-6 and before the container runtime;
- CENTRAL-2..6 runtime smokes remain wired in deterministic order;
- the CENTRAL-7 runtime closure runs after CENTRAL-6;
- Partner Portal MFA remains optional while the planned two_factor_authentication module remains in the module registry;
- Test/Golden partners remain visible in Partners while Dashboard/Impact excludes test partners from production impact aggregation;
- Module Registry categories remain authoritative for module records and package fixed module sets reference registry modules;
- Premium/Unlimited entitlement remains dynamic rather than tied to a fixed catalog cardinality;
- recurring invoices start as approval drafts and dunning/collection does not bypass the CENTRAL-6 invoice workflow;
- Partner Portal authentication remains gated by CENTRAL-6 onboarding ACTIVE + portal_enabled;
- invoice detail responses preserve the canonical payment/dunning fields used by the older invoice read model;
- no fixed 38/40 module-count contract is reintroduced.

Static acceptance reports all cross-contract failures together rather than stopping at the first drift.

## Runtime closure

scripts/smoke_central_7.sh runs after the individual CENTRAL-2..6 runtime smokes in the same Compose topology. It proves the integrated first-half state without redefining the individual feature tests:

1. public landing navigation exposes Partner Portal and Contact while Central Admin login is not advertised;
2. Dashboard still exposes monthly + weekly Impact and authoritative Recent Activity;
3. Golden/Test Partner remains visible with bilingual partner category data;
4. every module references an existing module group and the catalog remains cardinality-independent;
5. Starter/Business fixed module sets resolve to real registry modules and Premium remains Unlimited;
6. the CENTRAL-5 Premium acceptance partner retains all currently eligible published READY modules;
7. the CENTRAL-6 paid onboarding partner is ACTIVE, Portal-enabled, can authenticate without mandatory MFA, and exposes its paid invoice through the Partner Portal;
8. the CENTRAL-6 manual invoice round-trips the canonical payment fields including provider=MANUAL and a stored 72-hour payment deadline;
9. the CENTRAL-6 sponsored partner is ACTIVE with a documented waiver and no zero-dollar commercial invoice;
10. the finance overview remains ledger-backed after the full first-half runtime sequence.

## CI closure rule

CENTRAL-7 is accepted only when the same branch HEAD has:

- Go: SUCCESS
- Flutter: SUCCESS
- Compose: SUCCESS
- CENTRAL-7 static acceptance: SUCCESS
- CENTRAL-2..6 runtime acceptance: SUCCESS
- CENTRAL-7 runtime closure: SUCCESS

The branch is not mergeable as a completed CENTRAL-7 delivery until those gates are green on the same HEAD.
