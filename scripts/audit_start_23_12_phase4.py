#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

common = read("services/internal/common/common.go")
serviceauth = read("services/internal/serviceauth/contract.go")
gateway = read("services/cmd/gateway/main.go")
security = read("services/cmd/gateway/security_phase4.go")
partner = read("services/cmd/gateway/partner_portal.go")
main_dart = read("frontend/lib/main.dart")
partner_dart = read("frontend/lib/partner_portal.dart")
compose = read("docker-compose.yml")
render = read("render.yaml")
env_example = read(".env.example")
openapi = read("docs/openapi.yaml")

# Internal service trust: one transport signer, no feature-level signing maze.
require("func Sign(" in serviceauth and "func Verify(" in serviceauth, "central HMAC service identity contract is missing")
require("serviceauth.Sign(req, token, caller" in common, "DoInternal does not sign at the transport boundary")
require('strings.HasPrefix(r.URL.Path, "/internal/v1/")' in common and "serviceauth.Verify" in common,
        "InternalAuth does not verify signed internal routes")
require("HIMATE_REQUIRE_SERVICE_SIGNATURE" in common, "service signature enforcement switch is missing")
require("common.DoInternal(a.client,req)" in partner, "Partner Portal internal JSON calls bypass the common transport boundary")

manual_signing = []
for go_path in (ROOT / "services" / "cmd").rglob("*.go"):
    body = go_path.read_text()
    if "serviceauth.Sign(" in body:
        manual_signing.append(str(go_path.relative_to(ROOT)))
require(not manual_signing, "feature code manually signs service requests: " + ", ".join(manual_signing))

# External authority is reconstructed by Gateway/Partner session, never accepted from the client.
for header in [
    "X-Himate-User-ID", "X-Himate-Partner-ID", "X-Himate-Permissions",
    "X-Himate-Module-Keys", "X-Himate-Notification-Scope",
    "X-Himate-Caller-ID", "X-Himate-Caller-Timestamp", "X-Himate-Caller-Signature",
]:
    require(header in security, f"untrusted authority header is not stripped: {header}")
require("stripUntrustedAuthorityHeaders(r)" in gateway, "admin API does not strip client authority headers")
require("stripUntrustedAuthorityHeaders(r)" in partner, "Partner Portal API does not strip client authority headers")
require('r.Header.Set("X-Himate-Partner-ID",u.PartnerID)' in partner.replace(" ", ""),
        "Partner Portal does not reconstruct tenant authority from authenticated session")

# Browser protection deliberately preserves non-browser API/smoke compatibility.
require("browserMutationOriginAllowed" in security and "Sec-Fetch-Site" in security,
        "browser mutation origin policy is missing")
require("if !browserMutationOriginAllowed(r)" in gateway, "admin browser mutation policy is not wired")
require("if !browserMutationOriginAllowed(r)" in partner, "Partner browser mutation policy is not wired")
require("SameSite:http.SameSiteStrictMode" in gateway.replace(" ", ""), "admin session cookie is not SameSite Strict")
require("SameSite:http.SameSiteStrictMode" in partner.replace(" ", ""), "partner session cookie is not SameSite Strict")
require("len([]rune(password)) < 12" in gateway, "12-character password policy was weakened")

# MFA: production-only enforcement, no legacy CI coupling.
require("phase4MFAMigration" in security and "Version: 15" in security, "MFA migration is missing")
require("/api/v1/auth/mfa/verify" in gateway and "/partner/api/v1/auth/mfa/verify" in gateway,
        "MFA verify endpoints are missing")
require("verifyTOTP" in security and "attempts >= 5" in security, "TOTP or challenge attempt limit is missing")
require("promptMfaCode" in main_dart and "/api/v1/auth/mfa/verify" in main_dart,
        "admin MFA UI flow is missing")
require("/partner/api/v1/auth/mfa/verify" in partner_dart, "Partner Portal MFA UI flow is missing")
require('HIMATE_MFA_REQUIRED: "false"' in compose, "Compose must keep MFA disabled for inherited runtime compatibility")
require('key: HIMATE_MFA_REQUIRED' in render and 'value: "true"' in render,
        "Render production must require MFA")
require("HIMATE_MFA_REQUIRED=false" in env_example, "MFA environment contract is undocumented")

# Service signature enforcement is tested in Compose and production; caller identity uses no extra secret sprawl.
for caller in [
    "catalog","partners","billing","payments","contact","storage","runtime","backups",
    "evidence","impact","reports","cms","environments","connector","provisioning",
    "automation","tenantfinance","notifications","health",
]:
    require(f"HIMATE_SERVICE_CALLER_ID: {caller}" in compose, f"Compose caller identity missing: {caller}")
require(compose.count('HIMATE_REQUIRE_SERVICE_SIGNATURE: "${HIMATE_REQUIRE_SERVICE_SIGNATURE:-false}"') >= 19,
        "Compose must default to compatibility mode while allowing production-signature runtime override")
require(render.count("key: HIMATE_REQUIRE_SERVICE_SIGNATURE") >= 19 and render.count('value: "true"') >= 19,
        "Render does not enforce service signatures across private services")
require("HIMATE_SERVICE_CALLER_ID=gateway" in env_example, "service caller environment contract is undocumented")
require('"workshop": true' in serviceauth and '"scheduler": true' in serviceauth,
        "Phase 3B canonical Workshop/Scheduler producers are not recognized by the Phase 4 service-auth boundary")

# Existing moderate browser headers remain in place; Phase 4 does not break Flutter with a CSP rewrite.
for token in ["X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Strict-Transport-Security"]:
    require(token in gateway, f"security response header missing: {token}")

# Explicit non-goal: no blanket tenant FK/trigger firewall or business-gate rewrite in this rebuild.
require("phase4Tenant" not in gateway and "tenant ownership firewall" not in security.lower(),
        "blanket Phase 4 tenant firewall unexpectedly reintroduced")

# Public contract.
require("/api/v1/auth/mfa/verify:" in openapi and "/partner/api/v1/auth/mfa/verify:" in openapi,
        "OpenAPI MFA endpoints are missing")

print("START-23.12 Phase 4 security architecture audit: PASS")
