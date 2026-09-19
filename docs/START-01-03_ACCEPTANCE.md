# HIMATE START-01 to START-03 Acceptance Checklist

## START-01 - Project foundation

- [x] Separate `backend/` and `frontend/` application areas.
- [x] Go 1.23 backend builds and tests with the standard library only.
- [x] Flutter Web project shell is present and intentionally dependency-light.
- [x] Single-container Render deployment design is included.
- [x] Render service name is configured as `himate`.
- [x] Public health endpoint exists at `/api/v1/health`.
- [x] PostgreSQL foundation migrations are included for the permanent store contract.
- [x] Later START capabilities are explicitly reserved instead of partially implemented.

## START-02 - Authentication and admin shell

- [x] Bootstrap HIMATE administrator configuration through environment secrets.
- [x] Multiple bootstrap admins can be supplied with `HIMATE_BOOTSTRAP_ADMINS_JSON`.
- [x] Passwords are hashed immediately at process startup and are never returned by APIs.
- [x] Password derivation uses PBKDF2-HMAC-SHA256 with per-process random salt.
- [x] Signed, expiring HttpOnly session cookie.
- [x] Secure-cookie mode available and enabled by the production Docker image.
- [x] Authenticated `/auth/me` endpoint.
- [x] Logout clears the session cookie.
- [x] API error payloads are structured.
- [x] Baseline security headers and configurable CORS policy are present.
- [x] Flutter login screen and authenticated admin shell are present.

## START-03 - Navigation and card UX

- [x] Desktop side navigation.
- [x] Tablet/mobile drawer behavior.
- [x] Seven top-level HIMATE navigation areas from the CTO plan.
- [x] Dashboard card layout.
- [x] Responsive KPI layout.
- [x] Dashboard displays zero/not-collected states instead of invented business results.
- [x] Non-START-01/03 menu areas are visible only as explicit future-scope shells.
- [x] HIMATE navy/gold visual foundation and ascending-bar brand mark.

## Tests completed in this package

- Go formatting with `gofmt`.
- `go test ./...`.
- Auth password hashing tests.
- Session issue/verify/expiry tests.
- HTTP health test.
- HTTP login + authenticated dashboard test.
- Unauthorized dashboard test.

## Explicit boundary for the next round

The runtime identity store is intentionally an in-memory bootstrap store in START-01/03. A PostgreSQL schema contract is already supplied, but permanent PostgreSQL-backed user/session storage must be wired before the system is treated as persistent or production-ready. This avoids silently expanding the first development round into later database/infrastructure scope.
