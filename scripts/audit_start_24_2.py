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
partner = read("services/cmd/gateway/partner_portal.go")
modules = read("services/cmd/gateway/partner_user_modules.go")
notifications = read("services/cmd/notifications/main.go")
tenant_finance = read("services/cmd/tenantfinance/main.go")
tenant_storage = read("services/cmd/tenantfinance/storage.go")
smoke_design = read("scripts/smoke_start_23_11_4.sh")
smoke_modules = read("scripts/smoke_start_23_11_5.sh")
smoke_notifications = read("scripts/smoke_start_23_11_6.sh")

# Public callers cannot inject authority into the central or Partner Portal APIs.
require("stripUntrustedAuthorityHeaders(r)" in gateway,
        "central API no longer strips client-supplied authority headers")
require("stripUntrustedAuthorityHeaders(r)" in partner,
        "Partner Portal API no longer strips client-supplied authority headers")

# Tenant authority is always reconstructed from the authenticated Partner Portal identity.
require('r.Header.Set("X-Himate-Partner-ID",u.PartnerID)' in partner.replace(" ", ""),
        "Partner Portal does not reconstruct tenant authority from the authenticated session")
for token in [
    '"/api/v1/partners/"+url.PathEscape(u.PartnerID)',
    '"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)',
    '"/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)',
]:
    require(token in partner, f"Partner Portal tenant-scoped downstream route missing: {token}")

# User administration and module assignment remain tenant-keyed at the database boundary.
require("WHERE id=$1 AND partner_id=$2" in partner,
        "Partner Portal user mutation is not tenant-keyed")
require("WHERE id=$1 AND partner_id=$2" in modules,
        "Partner user module selection is not tenant-keyed")
require("MODULE_NOT_OWNED" in modules and "policy.Owned[moduleKey]" in modules,
        "module execution no longer intersects organization entitlement with user assignment")

# Tenant Finance receives partner identity from trusted gateway headers and scopes records by partner_id.
require("func partnerHeaders" in tenant_finance and 'r.Header.Get("X-Himate-Partner-ID")' in tenant_finance,
        "Tenant Finance trusted tenant-context reader is missing")
require("WHERE i.partner_id=$1" in tenant_storage,
        "Tenant Finance invoice reads are not partner-scoped")
require("WHERE id=$1 AND partner_id=$2" in tenant_storage,
        "Tenant Finance invoice mutations are not partner-scoped")
require("UNIQUE(partner_id,request_key)" in tenant_storage,
        "Tenant Finance idempotency is no longer tenant-scoped")

# Notification delivery is intersected with tenant, user, permission and module authority.
require("e.partner_id=$3" in notifications or "e.partner_id=$4" in notifications,
        "Partner notification SQL no longer filters partner_id")
require("event.PartnerID" in notifications and "event.TargetUserID" in notifications,
        "notification visibility no longer checks tenant/user delivery scope")
require("AudiencePermission" in notifications and "ModuleKey" in notifications,
        "notification visibility no longer intersects permission/module scope")

# Preserve executable cross-tenant regression proofs from START-23.11.x.
require("cross-tenant custom module icon references are rejected" in smoke_design,
        "cross-tenant design/media regression proof is missing")
require("cross-tenant Partner Portal user IDs are not addressable" in smoke_modules,
        "cross-tenant Partner Portal user regression proof is missing")
require("tenant B cannot read tenant A events" in smoke_notifications,
        "cross-tenant notification regression proof is missing")

print("START-24.2 RBAC/tenant-isolation acceptance: PASS")
