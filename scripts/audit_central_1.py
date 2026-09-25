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

marketing_pages = [
    "frontend/web/landing.html",
    "frontend/web/platform.html",
    "frontend/web/modules.html",
    "frontend/web/programs.html",
    "frontend/web/impact.html",
    "frontend/web/partners.html",
]

for path in marketing_pages:
    body = read(path)
    require('href="/partner/login">Partner Portal</a>' in body, f"{path} missing Partner Portal entry")
    require('class="nav-login-text" href="/contact">Contact</a>' in body, f"{path} missing Contact header action")
    require('class="nav-login-text" href="/login"' not in body, f"{path} still exposes Central login")
    require('class="login-pill" href="/login"' not in body, f"{path} still exposes Central login pill")

contact = read("frontend/web/contact.html")
for token in [
    'name="organization_type"',
    'name="inquiry_topic"',
    'value="PIANO_TECHNOLOGY"',
    'value="PRICING_LICENSING"',
    "organization_type: String(data.get('organization_type')",
    "inquiry_topic: String(data.get('inquiry_topic')",
]:
    require(token in contact, f"public contact form missing {token}")

for smoke_path in [
    "scripts/smoke_business_completion.sh",
    "scripts/smoke_start_23_1_mutation_canary.sh",
    "scripts/smoke_start_23_6.sh",
    "scripts/smoke_start_23_9.sh",
]:
    smoke = read(smoke_path)
    require('"organization_type"' in smoke, f"{smoke_path} still uses the legacy unclassified Contact contract")
    require('"inquiry_topic"' in smoke, f"{smoke_path} still uses the legacy unclassified Contact contract")

backend = read("services/cmd/contact/main.go")
for token in [
    'Version: 3, Name: "central-1-contact-classification"',
    'organization_type TEXT NOT NULL',
    'inquiry_topic TEXT NOT NULL',
    '"organization_type": item.OrganizationType',
    '"inquiry_topic": item.InquiryTopic',
    'ORGANIZATION_TYPE',
    'INQUIRY_TOPIC',
    'in.OrganizationType, in.InquiryTopic',
]:
    require(token in backend, f"contact backend missing {token}")

central = read("frontend/lib/contact_leads.dart")
require("lead['organization_type']" in central, "Central Contact Leads does not show organization type")
require("lead['inquiry_topic']" in central, "Central Contact Leads does not show inquiry topic")
require("Name, organization, type, topic, email or message" in central,
        "Central Contact Leads search does not advertise classification search")

gateway = read("services/cmd/gateway/main.go")
security = read("services/cmd/gateway/security_phase4.go")
partner = read("services/cmd/gateway/partner_portal.go")
frontend = read("frontend/lib/main.dart")

require('a.beginMFAFlow(w, r, "ADMIN", u.ID, in.Remember, true)' in gateway,
        "Central administrator MFA must remain enabled")
require("func partnerMFARequired(_ string) bool" in security and "return false" in security[security.index("func partnerMFARequired"):security.index("func (a *app) partnerMFAVerify")],
        "Partner Portal MFA must remain optional until module 40 activation")
require('a.beginMFAFlow(w,r,"PARTNER",u.ID,in.Remember,partnerMFARequired(u.Role))' in partner,
        "Partner MFA integration point was removed instead of being kept for module 40")
require("path == '/partner/login'" in frontend,
        "Partner Portal login route is not registered")
ssr_nav = gateway[gateway.index("func designNavigationHTML"):gateway.index("func renderSiteDesignHTML")]
require('href=\\\"/partner/login\\\"' in ssr_nav and 'Partner Portal' in ssr_nav,
        "Gateway SSR navigation does not render Partner Portal")
require('href=\\\"/contact\\\"' in ssr_nav and 'nav-login-text' in ssr_nav,
        "Gateway SSR navigation does not render Contact as the dedicated action")
require('href=\\\"/login\\\"' not in ssr_nav,
        "Gateway SSR navigation still exposes Central login")

print("Central-1 Landing/Contact/Partner-MFA acceptance: PASS")
