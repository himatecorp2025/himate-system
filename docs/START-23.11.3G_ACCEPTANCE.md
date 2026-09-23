# START-23.11.3g Acceptance — Partner Registration Error Handling

## Problem fixed

The New Partner backend previously used one condition for both JSON decoding and the required `display_name` check. Because partner payload decoding is strict (`DisallowUnknownFields`), frontend/backend version skew or any unknown field could fail JSON decoding but still be surfaced as **Display name is required**.

The frontend also closed the New Partner modal before the authoritative partner POST ran, so any backend failure appeared after the form had disappeared.

## Backend contract

- malformed or version-incompatible partner payloads return `400 JSON` with an **Invalid partner request** message;
- only a genuinely empty display name returns **Display name is required**;
- an unusable display-name slug receives its own validation;
- duplicate display-name-derived slug / primary-domain conflicts return `409` with an explicit conflict message.

## Modal contract

- validation errors are rendered inside the New Partner modal;
- the relevant Stepper step is selected when validation fails;
- the modal stays open while the request is submitted;
- authoritative API errors are rendered inside the modal instead of a bottom-page snackbar;
- the modal closes only after partner master data, first Portal Owner, selected logo and commercial defaults complete successfully;
- if the core partner was already created before a later onboarding step failed, **Retry setup** updates/reuses that same partner ID and does not create a duplicate.

## Version-skew implication

If a deployed frontend sends fields that an older `himate-partners` service does not yet understand, the UI now exposes that as a request-contract/backend-version problem rather than falsely blaming Display name. Deploying all affected monorepo services to the same release remains required.

## Automated evidence

- `python3 scripts/audit_start_23_11_3g.py`
- `sh scripts/smoke_start_23_11_3g.sh http://127.0.0.1:8080`
- full Flutter analyze/test/build, Go test/race/build and Compose regression suite remain mandatory.

Release contract: `0.8.24-start-23.11.3g`.

## START-23.11.3h supersession

START-23.11.3h intentionally supersedes the earlier display-name-derived slug conflict rule. Partner display names are presentation data and may repeat. HIMATE now generates the unique technical slug from the display name plus immutable partner ID. Primary-domain uniqueness remains independently enforced.
