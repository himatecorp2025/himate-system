# START-23.11.3h Acceptance — Test Partner Onboarding Hardening

## Scope freeze

START-23.11.4 remains out of scope. This patch closes New Partner onboarding so a real test partner can be created and verified in production before Partner Workspace & Personalization work begins.

## Display-name contract

- `display_name` is presentation data shown in HIMATE and Partner Portal.
- Two or more partners may use the same `display_name`.
- The display name is not an identity key and is not required to be unique.
- HIMATE allocates the immutable `ptr_......` partner ID independently.
- The internal technical slug is derived from the display name plus the immutable partner ID, so duplicate names do not collide.
- `primary_domain`, when supplied, remains independently unique.

## Retry and transaction-safety contract

The New Partner flow uses one stable `onboarding_request_id` for the lifetime of the modal. Replaying the same authoritative partner POST returns the already-created partner instead of creating another record.

Once the core partner exists:

- the modal cannot be dismissed with X, Cancel, route back or barrier interaction while onboarding is incomplete;
- Retry reuses the same partner ID;
- the first Partner Portal Owner is reconciled through the server-side user list before another POST is attempted;
- an already-linked partner logo is recognized from persisted `logo_url`;
- persisted billing terms are read and compared before another PUT is issued;
- the modal closes only after the complete requested onboarding state is confirmed.

## Production-test objective

After CI and controlled deployment, the operator must be able to create a test partner with any non-empty company display name, including a name already used by another partner. The resulting partner must have a distinct immutable partner ID and technical slug.

## Automated evidence

- `go test ./...` and `go test -race ./...`
- `flutter analyze`, Flutter Chrome tests and release build
- `python3 scripts/audit_start_23_11_3h.py`
- `sh scripts/smoke_start_23_11_3h.sh http://127.0.0.1:8080`
- full historical START regression suite through START-23.11.3h

Release contract: `0.8.25-start-23.11.3h`.
