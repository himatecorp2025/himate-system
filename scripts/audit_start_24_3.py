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
gateway_tests = read("services/cmd/gateway/main_test.go")
common = read("services/internal/common/common.go")
cms = read("services/cmd/cms/main.go")
cms_design = read("services/cmd/cms/design.go")
cms_tests = read("services/cmd/cms/main_test.go")
security = read("services/cmd/gateway/security_phase4.go")

# JSON entry points are bounded and reject unknown/trailing data.
require("io.LimitReader(r.Body, 1<<20)" in common,
        "shared JSON decoder no longer has a 1 MiB request-body bound")
require("decoder.DisallowUnknownFields()" in common,
        "shared JSON decoder accepts unknown fields")
require("request body must contain exactly one JSON value" in common,
        "shared JSON decoder no longer rejects trailing JSON values")

# Browser mutations remain exact-origin protected.
require("browserMutationOriginAllowed" in security and "Sec-Fetch-Site" in security,
        "browser origin/Fetch-Metadata validation is missing")
require("strings.EqualFold(u.Host, r.Host)" in gateway,
        "same-origin validation no longer compares request authority")

# Public URL construction must not trust client-controlled X-Forwarded-Host.
public_origin_start = gateway.index("func publicOrigin")
public_origin_end = gateway.index("func replaceHeadTag", public_origin_start)
public_origin = gateway[public_origin_start:public_origin_end]
require('r.Header.Get("X-Forwarded-Host")' not in public_origin,
        "public origin trusts X-Forwarded-Host")
require("r.Host" in public_origin,
        "public origin no longer uses edge-selected Host authority")
require("TestSTART243PublicOriginIgnoresUntrustedForwardedHost" in gateway_tests,
        "forwarded-host poisoning regression test is missing")

# Stored/public CMS links only allow site-relative paths or absolute HTTP(S) URLs.
require("func safeCTA" in cms and 'u.Scheme=="https"||u.Scheme=="http"' in cms.replace(" ", ""),
        "CMS safe link scheme validation is missing")
require("if !safeCTA(s.CTAURL)" in cms,
        "CMS section CTA validation no longer uses the safe URL policy")
require("!safeCTA(item.URL)" in cms_design,
        "site navigation validation no longer uses the safe URL policy")
require("javascript:alert(1)" in cms_tests and "data:text/html" in cms_tests,
        "unsafe URL scheme regression tests are missing")

# Dynamic SSR text/attribute values remain HTML escaped.
require("html.EscapeString(section.Heading)" in gateway,
        "dynamic CMS heading output is not escaped")
require("html.EscapeString(section.Body)" in gateway,
        "dynamic CMS body output is not escaped")
require("html.EscapeString(section.CTAURL)" in gateway,
        "dynamic CMS CTA URL output is not escaped")

# Baseline browser security headers stay active without changing the existing Flutter CSP model.
for token in [
    "X-Content-Type-Options",
    "X-Frame-Options",
    "Content-Security-Policy",
    "Strict-Transport-Security",
    "Referrer-Policy",
    "Permissions-Policy",
]:
    require(token in gateway, f"security response header missing: {token}")

print("START-24.3 API/input/web security acceptance: PASS")
