#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

gateway = read("services/cmd/gateway/main.go")
security = read("services/cmd/gateway/security_phase4.go")
partner = read("services/cmd/gateway/partner_portal.go")
gateway_tests = read("services/cmd/gateway/main_test.go")
security_tests = read("services/cmd/gateway/security_phase4_test.go")
partner_tests = read("services/cmd/gateway/partner_portal_test.go")

admin_mfa = 'a.beginMFAFlow(w, r, "ADMIN", u.ID, in.Remember, true)'
require(admin_mfa in gateway, "admin login no longer routes through MFA")
require(gateway.index(admin_mfa) < gateway.index("a.clearLoginFailures(key)", gateway.index(admin_mfa)),
        "admin login throttle is cleared before MFA succeeds")

partner_mfa = 'a.beginMFAFlow(w,r,"PARTNER",u.ID,in.Remember,partnerMFARequired(u.Role))'
require(partner_mfa in partner, "partner login no longer routes sensitive roles through MFA")
require(partner.index(partner_mfa) < partner.index("a.clearLoginFailures(key)", partner.index(partner_mfa)),
        "partner login throttle is cleared before MFA succeeds")

require(security.count("a.recordLoginFailure(key, now)") >= 2,
        "failed MFA verification is not tied back to authentication throttling")
require(security.count("a.clearLoginFailures(key)") >= 2,
        "successful MFA verification does not clear authentication throttling")
require("attempts >= 5" in security and "INTERVAL '10 minutes'" in security,
        "MFA challenge attempt or expiry controls are missing")
require("SameSite:http.SameSiteStrictMode" in security.replace(" ", ""),
        "MFA-issued session cookie is not SameSite Strict")
require("HttpOnly:true" in security.replace(" ", ""),
        "MFA-issued session cookie is not HttpOnly")

require("partnerAuthenticationStateChanged" in partner,
        "partner authentication-state rotation helper is missing")
require("!strings.EqualFold(current.Email,next.Email)" in partner.replace(" ", ""),
        "partner email changes no longer invalidate existing sessions")
require("ifpartnerAuthenticationStateChanged(current,next){sessionVersion++}" in partner.replace(" ", ""),
        "partner authentication-state changes do not rotate session version")

require("TestSTART241PartnerAuthenticationStateChangeRotatesSession" in partner_tests,
        "partner session-rotation regression test is missing")
require("TestPhase4TOTPVerificationWindow" in security_tests,
        "TOTP verification regression test is missing")
require("TestSTART241" in security_tests or "MFA" in security_tests,
        "MFA throttling regression coverage is missing")

print("START-24.1 authentication/session/MFA acceptance: PASS")
