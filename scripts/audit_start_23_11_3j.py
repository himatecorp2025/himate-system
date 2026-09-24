#!/usr/bin/env python3
from pathlib import Path
import sys

root = Path(__file__).resolve().parents[1]
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
catalog = (root / "services/cmd/catalog/main.go").read_text(encoding="utf-8")
portal = (root / "services/cmd/catalog/partner_portal.go").read_text(encoding="utf-8")
billing = (root / "services/cmd/billing/main.go").read_text(encoding="utf-8")
impact = (root / "services/cmd/impact/main.go").read_text(encoding="utf-8")
frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")
compose = (root / "docker-compose.yml").read_text(encoding="utf-8")

release = "0.8.32-start-23.11.7"

checks = [
    (
        "partner master model has explicit Golden Test flag",
        "test_partner BOOLEAN NOT NULL DEFAULT FALSE" in partners
        and "TestPartner" in partners
        and 'json:"test_partner"' in partners
        and '"test_partner": p.TestPartner' in partners,
    ),
    (
        "Golden Test activation moves tenant to LIVE without billing/provisioning gate",
        'goldenActivation := !oldTestPartner && p.TestPartner' in partners
        and 'p.Lifecycle = "LIVE"' in partners
        and "!goldenActivation && requiresProvisioningGate" in partners,
    ),
    (
        "Golden Test flag cannot be casually removed and contaminate real aggregates",
        "GOLDEN_TEST_PARTNER_IMMUTABLE" in partners
        and "dedicated test-tenant purge workflow" in partners
        and "partner['test_partner'] == true" in frontend,
    ),
    (
        "Golden Test receives every canonical module as active entitlement",
        "quote_reference='GOLDEN-TEST-PARTNER'" in catalog
        and "entitlement_source='TEST'" in catalog
        and "plan_key='GOLDEN_TEST'" in catalog
        and "SELECT module_key FROM catalog.modules WHERE system=TRUE" in catalog,
    ),
    (
        "Partner marketplace exposes canonical module set in test mode",
        'visibilityClause = " AND m.system=TRUE"' in portal
        and 'item.AccessState = "ACTIVE"' in portal
        and "if testPartner" in portal,
    ),
    (
        "test billing data is excluded from platform revenue analytics",
        billing.count("p.test_partner=FALSE") >= 2,
    ),
    (
        "test impact data is excluded only from global aggregates",
        impact.count("COALESCE(p.test_partner,FALSE)=FALSE") >= 3
        and 'if partnerID!="" {' in impact,
    ),
    (
        "partner workspace secondary loading is bounded",
        ".timeout(const Duration(seconds: 8))" in frontend
        and "timed out after 8 seconds" in frontend
        and "_supplementalLoadGeneration" in frontend,
    ),
    (
        "admin can mark and clearly identify Golden Test Partner",
        "Golden Test Partner" in frontend
        and "'test_partner': testPartner" in frontend
        and "GOLDEN TEST" in frontend
        and "TEST DATA · excluded from platform aggregates" in frontend,
    ),
    (
        "OpenAPI publishes Golden Test Partner contract",
        "test_partner:" in openapi
        and "Golden test tenant marker" in openapi
        and "version: " + release in openapi
        and "START-01 through START-23.11.6" in openapi,
    ),
    (
        "all application services share the START-23.11.3j release",
        render.count("value: " + release) == 19
        and compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-" + release + "}") == 18,
    ),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:", failure)
    sys.exit(1)

print("START-23.11.3j Golden Test Partner & Workspace Stabilization static audit: PASS")
