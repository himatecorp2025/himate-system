# START-23.11.3i Acceptance — Release Consistency & Partner Data Contract

## Scope

START-23.11.4 remains frozen. This patch closes the mixed-microservice-version failure mode observed during real New Partner testing and proves that the complete partner master-data payload is persisted end to end.

## Release consistency contract

- Every deployable HIMATE application service receives the same `HIMATE_APP_VERSION`.
- The shared HTTP layer publishes `X-Himate-App-Version` and `X-Himate-Service` on service responses.
- Gateway-to-service mutation traffic carries `X-Himate-Expected-Version`.
- A service with a missing or different release version rejects version-bound internal mutation traffic.
- The gateway independently preflights downstream release versions before mutations.
- New Partner creation preflights the complete onboarding dependency set: Partners, Billing, CMS and Storage.
- Partner logo onboarding preflights Partners, CMS and Storage.
- `/api/v1/health` exposes `service_versions`, `release_consistent`, and explicit `version_unknown` / `version_mismatch` states.
- A mixed deployment is therefore blocked with `RELEASE_MISMATCH` instead of being allowed to produce misleading field-validation errors.

## Partner master-data contract

The New Partner payload and authoritative Partners service agree on the following persisted fields:

- display name
- legal company name
- brand / DBA
- category and lifecycle
- primary domain
- registration number
- Tax / VAT ID
- country, state / region, city, postal code
- registered address lines 1 and 2
- website and phone
- primary / Portal Owner contact
- finance contact
- technical contact
- marketing contact
- internal notes
- stable onboarding request ID

Display name remains presentation data. Its only name-content invariant is non-empty after trimming; duplicate display names are valid because identity is provided by the immutable partner ID and collision-free technical slug.

The Partners service stores master data in PostgreSQL columns and returns the same authoritative fields on GET. The strengthened Compose acceptance performs a POST -> PostgreSQL -> GET roundtrip and compares every master-data field.

## Identity contract

Partner Portal Owner creation remains in the same onboarding flow. The acceptance suite creates an Owner and reads it back from the partner-scoped identity list.

## Deployment contract

Render automatic deployment remains disabled. A production release is valid only when every application service participating in the release has been deployed from the same release version.

Release contract: `0.8.26-start-23.11.3i`.

## Automated evidence

- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- Flutter analyze / Chrome tests / release build
- `python3 scripts/audit_start_23_11_3i.py`
- `sh scripts/smoke_start_23_11_3i.sh http://127.0.0.1:8080`
- all historical START acceptance gates through START-23.11.3i
