# START-23.6 Acceptance — Administration, Identity & Business CRUD Completion

## Purpose

START-23.6 closes the Administration / Identity / core-business mutation contracts that remained source-complete but production-unverified after START-23.5.

This phase does not redesign the product. It makes the existing administrator, partner, profile, notification and contact workflows end-to-end authoritative and removes login controls that are not backed by a real implementation.

## Authentication and identity contract

### Login and logout

- `POST /api/v1/auth/login` remains the authoritative administrator sign-in path.
- `GET /api/v1/auth/me` resolves the current database-backed identity and current `session_version`.
- `POST /api/v1/auth/logout` clears the HttpOnly session cookie.
- suspended users and superseded session versions are rejected on every authenticated request.

### Forgot Password

Public endpoints:

- `POST /api/v1/auth/password-reset/request`
- `POST /api/v1/auth/password-reset/confirm`

Required properties:

1. valid requests do not reveal whether an email address exists;
2. reset tokens contain at least 256 bits of random material;
3. plaintext reset tokens are never persisted;
4. the database stores only a SHA-256 token hash;
5. only one active reset generation is retained per administrator;
6. tokens expire after the configured TTL, default 30 minutes;
7. tokens are one-time-use and replay fails closed;
8. reset-request traffic is rate limited;
9. successful reset applies the existing password-complexity policy;
10. successful reset increments `identity.users.session_version`, invalidating all previously issued administrator sessions;
11. production delivery requires runtime SMTP and reset-base-URL configuration;
12. local/CI may expose the generated token only in the development response so the real reset path can be mutation-tested without an external mail provider;
13. reset request/completion generate central audit events.

### SSO

No OIDC/SAML provider is configured in START-23.6.

Therefore the previous non-functional **Sign in with SSO** control is removed from the login UI. A visible control may return only when a real provider-backed authentication flow exists. A placeholder snackbar is not accepted.

## Administrator and RBAC contract

### Custom roles

- system owner can create custom roles;
- a custom role can be assigned to administrators;
- backend permissions are resolved from the current role definition;
- changing a role's permission set affects an already authenticated user on the next authorization check;
- built-in roles remain protected;
- custom roles cannot be deactivated while assigned.

### Administrator users

- system owner can create and edit administrators;
- email uniqueness is enforced across HIMATE and Partner Portal identities;
- system-owner / Platform Admin protections remain enforced;
- authentication-affecting edits — email, role, active status, or password — rotate `session_version`;
- suspending an administrator invalidates an already issued session and denies subsequent sign-in.

## Partner business CRUD contract

The existing Partner service remains authoritative.

Required proof:

- create partner in `PROSPECT`;
- read the created record;
- update partner company/profile fields;
- perform an allowed lifecycle transition through the backend state machine;
- reload and prove persistence.

### Partner Portal identities

- HIMATE system owner can create a tenant-bound Partner Portal user;
- the created user can authenticate only into that partner tenant;
- role or active-state changes rotate Partner Portal `session_version`;
- an already issued Partner Portal session becomes invalid after role/status mutation;
- last-owner safeguards remain enforced.

## HIMATE company profile

`/api/v1/billing/profile` remains the single authoritative HIMATE company profile.

The Administration and Licensing & Finance surfaces must use the same backend record. START-23.6 acceptance mutates that profile, reads it back and restores the previous state.

## Profile and password

- `PATCH /api/v1/profile` persists the signed-in administrator's profile;
- email uniqueness remains enforced;
- locale/timezone remain normalized;
- `POST /api/v1/profile/password` checks the current password, applies the common password policy and rotates `session_version`;
- the current browser receives a replacement session while previously issued sessions are invalidated.

## Contact Leads and notifications

START-23.6 retains the already persisted Contact Leads workflow and proves it inside the Administration CRUD closure:

- public contact submission persists;
- administrator lead status/assignment/note update persists;
- notification read marks one notification read for the current administrator;
- read-all clears all visible unread notifications;
- central audit continues to capture authenticated mutations.

## Deployment contract

Production Gateway configuration declares:

- `HIMATE_PASSWORD_RESET_TTL_MINUTES`;
- `HIMATE_PASSWORD_RESET_BASE_URL`;
- `SMTP_HOST`;
- `SMTP_PORT`;
- `SMTP_USERNAME`;
- `SMTP_PASSWORD`;
- `SMTP_FROM`.

Secrets remain runtime-only.

Release contract version: `0.8.10-start-23.6`.

## Required automated proof

### Static audit

`scripts/audit_start_23_6.py` must verify at least:

- password reset API + hashed token persistence;
- session-version rotation for password reset and administrator auth mutations;
- production reset delivery configuration;
- reset UI wiring;
- absence of the SSO placeholder control;
- START-23.6 matrix completion;
- OpenAPI and release version;
- CI registration.

### Compose mutation smoke

`scripts/smoke_start_23_6.sh` must prove against real containers and PostgreSQL:

1. login and `/auth/me`;
2. logout invalidates the browser session;
3. forgot-password request returns a development one-time token in CI;
4. reset changes the password;
5. old authenticated session is rejected;
6. reset token replay is rejected;
7. new password authenticates;
8. owner password is restored through the normal profile-password path;
9. custom role create and permission enforcement;
10. custom role edit affects an existing administrator session;
11. administrator suspension invalidates the active session and blocks login;
12. partner create/read/edit/lifecycle transition;
13. Partner Portal user create and tenant login;
14. Partner Portal role/status mutation invalidates prior session;
15. authoritative HIMATE company profile mutation/readback/restore;
16. profile mutation/readback/restore;
17. public Contact inquiry + lead workflow update;
18. notification single-read and read-all persistence;
19. central audit contains the representative identity/admin/profile/contact mutations.

## Definition of Done

START-23.6 is complete only when:

- Go tidy/vet/unit/race/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23 through START-23.5 static audits remain green;
- START-23.6 static audit passes;
- complete START-01 through START-23.5 Compose regression remains green;
- START-23.6 mutation smoke passes;
- the functional matrix marks every START-23.6 contract as proven or intentionally hidden where no provider exists;
- the pull request is merged into `develop`.

After merge, a separate START-23.1–23.6 cross-phase audit is mandatory before START-23.7 begins.
