#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.6 audit failed: " + message)

def phase_tuple(value: str):
    return tuple(int(part) for part in str(value).split("."))

gateway = read("services/cmd/gateway/main.go")
gateway_tests = read("services/cmd/gateway/main_test.go")
frontend = read("frontend/lib/main.dart")
localization = read("frontend/lib/localization.dart")
compose = read("docker-compose.yml")
render = read("render.yaml")
openapi = read("docs/openapi.yaml")
ci = read(".github/workflows/ci.yml")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))

# Password reset: real API, hashed persistence, expiry and session rotation.
for token in (
    '/api/v1/auth/password-reset/request',
    '/api/v1/auth/password-reset/confirm',
    'identity.password_reset_tokens',
    'passwordResetTokenHash',
    'expires_at',
    'used_at',
    'session_version=session_version+1',
    'PASSWORD_RESET_REQUESTED',
    'PASSWORD_RESET_COMPLETED',
):
    require(token in gateway, f"gateway password-reset contract missing {token!r}")

require('sha256.Sum256([]byte(token))' in gateway, "reset token is not SHA-256 hashed before persistence")
require('newPasswordResetToken' in gateway and 'make([]byte, 32)' in gateway, "reset token does not use 32 random bytes")
require('a.env == "production" && !a.passwordResetDeliveryConfigured()' in gateway,
        "production password reset does not fail closed when delivery is unavailable")
require('authChanged := in.Email != nil || in.Roles != nil || in.Active != nil' in gateway,
        "administrator auth-affecting edits do not rotate session version")
require('if authChanged { sessionVersion++ }' in gateway,
        "administrator session-version rotation guard missing")
require('TestSTART236PasswordResetTokenHash' in gateway_tests,
        "password-reset token unit guard missing")

# Login UI: Forgot Password is wired; unavailable SSO is not presented as a fake action.
for token in (
    'onRequestPasswordReset',
    'onConfirmPasswordReset',
    'forgotPassword',
    'showResetPassword',
    '/api/v1/auth/password-reset/request',
    '/api/v1/auth/password-reset/confirm',
    "resetRequestTitle",
    "resetNewPasswordTitle",
):
    require(token in frontend or token in localization, f"frontend reset flow missing {token!r}")

for stale in ('Sign in with SSO', 'ssoPending', 'onSso', 'recoveryPending',
              'Password recovery will be connected in the security phase'):
    require(stale not in frontend and stale not in localization,
            f"stale login placeholder remains: {stale!r}")

# Deployment/runtime configuration.
for token in (
    'HIMATE_PASSWORD_RESET_TTL_MINUTES',
    'HIMATE_PASSWORD_RESET_BASE_URL',
    'SMTP_HOST',
    'SMTP_PORT',
    'SMTP_USERNAME',
    'SMTP_PASSWORD',
    'SMTP_FROM',
):
    require(token in render, f"Render Gateway reset configuration missing {token}")
    require(token in compose, f"Compose Gateway reset configuration missing {token}")

require('HIMATE_APP_VERSION' in render and 'start-23.' in render, "Render release contract is missing")
require('HIMATE_APP_VERSION' in compose and 'start-23.' in compose, "Compose release contract is missing")
require('version: 0.8.' in openapi and '-start-23.' in openapi, "OpenAPI release contract is missing")
for path in ('/api/v1/auth/password-reset/request:', '/api/v1/auth/password-reset/confirm:'):
    require(path in openapi, f"OpenAPI missing {path}")

# Functional matrix: every 23.6 contract must be explicitly closed with proof.
require(phase_tuple(matrix.get("completed_through", "0")) >= phase_tuple("23.6"), "functional matrix is not completed through 23.6")
rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") == "23.6"]
require(rows, "functional matrix contains no START-23.6 contracts")
for row in rows:
    state = str(row.get("current_state", ""))
    proof = str(row.get("e2e_proof", ""))
    require("MISSING" not in state and "PLACEHOLDER" not in state,
            f"{row.get('id')} still has incomplete state {state!r}")
    require("smoke_start_23_6.sh" in proof or row.get("id") == "AUTH-SSO",
            f"{row.get('id')} lacks START-23.6 E2E proof")

# CI and acceptance registration.
require('docs/START-23.6_ACCEPTANCE.md' in ci, "CI does not require START-23.6 acceptance document")
require('audit_start_23_6.py' in ci, "CI does not execute START-23.6 static audit")
require('smoke_start_23_6.sh' in ci, "CI does not execute START-23.6 Compose smoke")

print(f"START-23.6 static audit passed ({len(rows)} functional contracts)")
