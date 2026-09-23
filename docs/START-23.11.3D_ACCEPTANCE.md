# START-23.11.3d Acceptance — Real Partner Onboarding

## Decision

The fixed HIMATE TEST PARTNER fixture is retired.

Manual QA now uses the same partner-creation path as a real customer. The HIMATE New Partner wizard creates the partner record and immediately registers the first Partner Portal Owner using the administrator/contact email and an explicitly entered initial password.

## New Partner contract

The wizard requires:
- display name;
- primary contact / Portal owner name;
- administrator / Partner Portal email;
- initial Partner Portal password;
- the existing commercial/provisioning requirements.

After the partner record is created, HIMATE creates:
- one Partner Portal user;
- role: `owner`;
- partner scope: the newly allocated partner ID;
- email: the administrator/contact email;
- password: hashed by the existing Partner Portal password pipeline.

The administrator can then sign in at `/partner/login`.

## Fixed fixture retirement

Forward database migrations remove the previous fixed QA records from long-lived deployments:
- partners migration version 6 removes `ptr_himate_test_001`;
- identity migration version 12 removes its Partner Portal identity.

No startup seed recreates either record.

## Partner Login password-field fix

The password field remains password-manager compatible, but now also exposes an explicit **Clear password** control.

Clearing:
- ends the active autofill context without saving;
- clears the controller value;
- keeps focus in the password field;
- clears stale login error text.

This prevents browser autofill from making an entered password appear impossible to remove.

## Diagnostics

Partner Portal business endpoints use the precise partner-registry error classifier rather than masking registry failures as a generic access-disabled 403.

## Automated evidence

- Flutter widget test: `frontend/test/partner_portal_login_test.dart`;
- static audit: `python3 scripts/audit_start_23_11_3d.py`;
- Compose smoke: `sh scripts/smoke_start_23_11_3d.sh http://127.0.0.1:8080`.

The smoke verifies that the old fixed fixture is absent, creates a completely new partner through the normal admin API, creates its first Owner account, and successfully performs a real Partner Portal login.

Release contract: `0.8.21-start-23.11.3d`.
