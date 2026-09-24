#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
common = (root / "services/internal/common/common.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
compose = (root / "docker-compose.yml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
workflow = (root / ".github/workflows/ci.yml").read_text(encoding="utf-8")
fast_workflow = (root / ".github/workflows/ci-fast.yml").read_text(encoding="utf-8")
schema_guard = (root / "scripts/audit_smoke_schema_contracts.py").read_text(encoding="utf-8")
execution_guard = (root / "scripts/audit_smoke_execution_contracts.py").read_text(encoding="utf-8")
release_smoke = (root / "scripts/smoke_start_23_11_3i.sh").read_text(encoding="utf-8")

release = "0.8.29-start-23.11.4"
frontend_start = frontend.index("  Future<void> addPartner() async {")
frontend_end = frontend.index("  List<Map<String, dynamic>> get filtered => partners;", frontend_start)
add_partner = frontend[frontend_start:frontend_end]

cms_fetch_start = gateway.index("func (a *app) fetchPublishedCMS")
cms_fetch_end = gateway.index("func publicOrigin", cms_fetch_start)
cms_fetches = gateway[cms_fetch_start:cms_fetch_end]

notification_start = gateway.index("func (a *app) emitNotification")
notification_end = gateway.index("func auditLimit", notification_start)
notification_dispatch = gateway[notification_start:notification_end]

master_fields = [
    "display_name","legal_name","brand_name","category_id","lifecycle","primary_domain",
    "contact_name","contact_email","finance_contact_name","finance_contact_email",
    "technical_contact_name","technical_contact_email","marketing_contact_name","marketing_contact_email",
    "registration_number","tax_id","country","state_region","city","postal_code",
    "address_line1","address_line2","website","phone","notes",
]

service_go_files = [
    path for path in (root / "services/cmd").rglob("*.go")
    if not path.name.endswith("_test.go") and "gateway" not in path.parts
]
raw_internal_header_writes = []
for path in service_go_files:
    source = path.read_text(encoding="utf-8")
    if '.Header.Set("X-Himate-Internal-Token"' in source:
        raw_internal_header_writes.append(str(path.relative_to(root)))

checks = [
    (
        "shared HTTP layer publishes and enforces release version",
        "func ReleaseGuard(service string, next http.Handler) http.Handler" in common
        and "X-Himate-App-Version" in common
        and "X-Himate-Expected-Version" in common
        and '"RELEASE_MISMATCH"' in common,
    ),
    (
        "shared internal-request helper binds credentials and expected release",
        "func BindInternalRequest(req *http.Request, token string)" in common
        and 'req.Header.Set("X-Himate-Expected-Version", version)' in common
        and not raw_internal_header_writes,
    ),
    (
        "unversioned application services cannot enter the serving loop",
        'if AppVersion() == "" {' in common
        and 'HIMATE_APP_VERSION is required' in common,
    ),
    (
        "gateway preflights release compatibility before mutations",
        "func (a *app) requireServiceReleases" in gateway
        and "a.requireServiceReleases(w, r, service)" in gateway
        and 'a.requireServiceReleases(w, r, "partners", "billing", "cms", "storage")' in gateway,
    ),
    (
        "gateway health reports cross-service release consistency",
        '"service_versions": serviceVersions' in gateway
        and '"release_consistent": overall == "ok"' in gateway
        and '"version_mismatch"' in gateway
        and '"version_unknown"' in gateway,
    ),
    (
        "gateway direct internal calls are version-bound too",
        "common.BindInternalRequest(req,a.internalToken)" in (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
        and "common.BindInternalRequest(req, a.internalToken)" in (root / "services/cmd/gateway/partner_branding.go").read_text(encoding="utf-8")
        and "common.DoInternal(a.client, req)" in (root / "services/cmd/gateway/partner_branding.go").read_text(encoding="utf-8"),
    ),
    (
        "gateway SSR CMS reads are version-bound and validate downstream release",
        'Header.Set("X-Himate-Internal-Token"' not in cms_fetches
        and cms_fetches.count("common.BindInternalRequest(req, a.internalToken)") == 6
        and cms_fetches.count("common.DoInternal(a.client, req)") == 6,
    ),
    (
        "gateway notification dispatch is version-bound",
        "common.BindInternalRequest(req,a.internalToken)" in notification_dispatch
        and "common.DoInternal(a.client,req)" in notification_dispatch
        and 'Header.Set("X-Himate-Internal-Token"' not in notification_dispatch,
    ),
    (
        "feature branches use fast checkpoint CI while full acceptance is reserved for merge gates",
        "branches: [develop, main]" in workflow
        and "'start-*'" not in workflow.split("pull_request:", 1)[0]
        and "'pre-start-*'" not in workflow.split("pull_request:", 1)[0]
        and "name: HIMATE Fast CI" in fast_workflow
        and "branches: ['start-*', 'pre-start-*']" in fast_workflow
        and "compose-current:" in fast_workflow
        and "smoke_start_23_11_3h.sh" in fast_workflow
        and "smoke_start_23_11_3i.sh" in fast_workflow,
    ),
    (
        "smoke schema drift is rejected before expensive Compose build",
        "Validate smoke database schema contracts" in workflow
        and "python3 scripts/audit_smoke_schema_contracts.py" in workflow
        and workflow.index("Validate smoke database schema contracts") < workflow.index("Build and start containerized microservices")
        and "identity.partner_users" in schema_guard
        and '"role", "role_key"' in schema_guard,
    ),
    (
        "smoke shell execution is validated before expensive Compose build",
        "Validate smoke shell execution contracts" in workflow
        and "python3 scripts/audit_smoke_execution_contracts.py" in workflow
        and workflow.index("Validate smoke shell execution contracts") < workflow.index("Build and start containerized microservices")
        and "Smoke shell execution contract audit: PASS" in execution_guard
        and "direct exec of a .sh file is not portable" in execution_guard,
    ),
    (
        "START-23.11.3i release smoke is standalone and non-duplicative",
        "gateway reports one synchronized application release" in release_smoke
        and "release_consistent" in release_smoke
        and "RELEASE_MISMATCH" in release_smoke
        and "X-Himate-Expected-Version" in release_smoke
        and "smoke_start_23_11_3h.sh" not in release_smoke,
    ),
    (
        "Compose pins one current release across every application microservice",
        compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-" + release + "}") == 18,
    ),
    (
        "Render pins one current release across every deployable application service",
        render.count("value: " + release) == 19
        and render.count("autoDeploy: false") == 19,
    ),
    (
        "New Partner frontend carries the complete master-data contract",
        all("'" + field + "'" in add_partner for field in master_fields),
    ),
    (
        "partners backend accepts the complete master-data JSON contract",
        all('json:"' + field + '"' in partners for field in master_fields),
    ),
    (
        "partners backend persists and returns all master-data fields",
        all('"' + field + '"' in partners for field in master_fields)
        and "registration_number,tax_id,country,state_region,city,postal_code,address_line1,address_line2,website,phone,notes,onboarding_request_id" in partners,
    ),
    (
        "display name has no format or uniqueness restriction beyond non-empty presentation value",
        'if in.DisplayName == "" {' in partners
        and "partnerTechnicalSlug(in.DisplayName, id)" in partners
        and "Display name must contain at least one letter or number" not in partners
        and "display-name slug or primary domain is already in use" not in partners,
    ),
    (
        "partner updates preserve the same non-empty display-name invariant",
        'if p.DisplayName == "" {' in partners,
    ),
    (
        "release contract",
        "version: " + release in openapi
        and "START-01 through START-23.11.3k" in openapi,
    ),
]

failures = [label for label, ok in checks if not ok]
if raw_internal_header_writes:
    print("FAIL: raw internal-token header writes bypass shared release binding:", ", ".join(raw_internal_header_writes))

if failures:
    for failure in failures:
        print("FAIL:", failure)
    raise SystemExit(1)

print("START-23.11.3i Release Consistency & Partner Contract static audit: PASS")
