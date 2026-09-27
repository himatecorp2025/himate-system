#!/usr/bin/env python3
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]

def read(path:str)->str:
    return (ROOT/path).read_text()

def check(ok:bool,message:str)->None:
    if not ok:
        raise SystemExit("CENTRAL-14 FAIL: "+message)

frontend=read("frontend/lib/main.dart")
admin_ui=read("frontend/lib/administration_center.dart")
backup_ui=read("frontend/lib/backups_panel.dart")
localization=read("frontend/lib/localization.dart")
gateway=read("services/cmd/gateway/main.go")
gateway10=read("services/cmd/gateway/central10.go")
gateway14=read("services/cmd/gateway/central14.go")
step4=read("services/cmd/gateway/central_step4_snapshots.go")
backups_main=read("services/cmd/backups/main.go")
backups_api=read("services/cmd/backups/api.go")
backups_artifact=read("services/cmd/backups/artifact.go")
restore=read("services/cmd/backups/central14_restore.go")
storage=read("services/cmd/storage/main.go")
billing=read("services/cmd/billing/main.go")
billing14=read("services/cmd/billing/central14.go")
environments=read("services/cmd/environments/main.go")
runtime=read("services/cmd/runtime/main.go")
openapi=read("docs/openapi.yaml")
acceptance=read("docs/CENTRAL-14_ACCEPTANCE.md")

for token in [
    "'HIMATE Administration Center'",
    "'Partner Administration Center'",
    "title: 'Administration'",
    "const LinearProgressIndicator(",
    "section = 'root'",
    "title: 'Financial Administration'",
    "title: 'Corporate Documents'",
    "title: 'Governance, Settings & Access'",
    "title: 'System Backup & Recovery'",
    "title: 'Audit & Logs'",
    "title: 'Backup & Recovery'",
]:
    check(token in admin_ui, f"Administration center workspace missing: {token}")

check("AdministrationPage(api: widget.api, user: widget.user, onBack:" in admin_ui,
      "legacy governance/RBAC workspace is not preserved as a submodule")
check("part 'administration_center.dart';" in frontend,
      "Administration Center frontend part is not registered")

for token in [
    "func (a *app) materializeCentralAdministration(ctx context.Context)",
    "func (a *app) central14Administration(",
    'a.hasPermission(actor, "partners.read")',
    'a.hasPermission(actor, "billing.read")',
    'a.hasPermission(actor, "backups.read")',
    'a.hasPermission(actor, "audit.read")',
    "centralSnapshotForRead(r.Context(), centralStep4AdministrationKey)",
    'centralStep4Meta(started, centralStep4AdministrationKey, updatedAt, "healthy", []string{})',
]:
    check(token in gateway14, f"Administration authoritative read-model/RBAC contract missing: {token}")
for token in [
    'centralStep4AdministrationKey = "administration_screen"',
    "refreshCentralStep4Administration",
]:
    check(token in step4, f"Administration materializer contract missing: {token}")
check('case path == "/api/v1/central/administration":' in gateway and 'return "administration"' in gateway,
      "Administration read model is not protected by administration RBAC")
check('a.central14Administration(w, r, u)' in gateway,
      "Administration read model route missing")

for token in [
    'mux.HandleFunc("/api/v1/billing/company/documents", a.companyDocuments)',
    'mux.HandleFunc("/internal/v1/administration/summary", a.central14AdministrationSummary)',
    'LOWER(name) LIKE',
    'LOWER(kind) LIKE',
    'LOWER(note) LIKE',
    'LOWER(storage_url) LIKE',
]:
    check(token in billing, f"Administration document contract missing: {token}")
check('central14CompanyDocumentScope = "_himate"' in billing14,
      "company document scope missing")

for token in [
    "platformDBName string",
    "VALUES('_platform',30,30,24,TRUE)",
]:
    check(token in backups_main, f"platform backup scope missing: {token}")
for token in [
    'if partnerID=="_platform"{dbName=strings.TrimSpace(a.platformDBName)}',
    'if partnerID=="_platform"{',
    "restored platform identity schema is missing",
]:
    check(token in backups_artifact, f"platform backup/restore-test contract missing: {token}")
check("platform production restore is maintenance-only" in restore,
      "live platform replacement is not explicitly blocked")

for token in [
    "func (a *app) verifiedRestorePoint(",
    "PASSED restore test before production restore",
    'confirmation must exactly match RESTORE %s',
    "restore already queued or running for partner",
    "func (a *app) createSafetyRestorePoint(",
    "func (a *app) rollbackToSafetyRestorePoint(",
    "automatic safety rollback PASSED",
    "automatic safety rollback FAILED",
    "func (a *app) suspendPartnerEnvironments(",
    "func (a *app) restoreEnvironmentRelease(",
]:
    check(token in restore, f"verified production restore safety gate missing: {token}")
for token in [
    'parts[2]=="restore"',
    'parts[0]=="restores"',
]:
    check(token in backups_api, f"production restore API route missing: {token}")

for token in [
    "func (a *app) restoreNamespace(",
    "Unsafe media restore path",
    "Media restore path escapes namespace",
    "Media restore archive contains unsupported entry type",
    'case r.Method == http.MethodPost && action == "restore-archive":',
]:
    check(token in storage, f"atomic media restore contract missing: {token}")

for token in [
    '"/internal/v1/environments/recovery-release"',
    '"operation_id":operationID',
    '"allow_reuse":allowReuse',
    'jsonEquivalent(current["config"],env["config"])',
    '"restore:"+job.ID',
    '"rollback:"+job.ID',
]:
    check(token in restore, f"captured runtime release recovery missing: {token}")
for token in [
    'mux.HandleFunc("/internal/v1/environments/recovery-release", a.recoveryRelease)',
    "func (a *app) finalizeRecoveryDeployment(",
    "func (a *app) deployRecordWithOperation(",
    "environment_status='SUSPENDED'",
    'payload["operation_id"]=operationID',
]:
    check(token in environments, f"recovery-safe environment orchestration missing: {token}")
for token in [
    "func deploymentRequestKeyWithOperation(",
    "OperationID string",
    "requestKey:=deploymentRequestKeyWithOperation(",
    '"operation:"+operationID',
]:
    check(token in runtime, f"recovery deployment idempotency missing: {token}")
check("if r.Method==http.MethodGet" in environments and 'common.JSON(w,http.StatusOK,mapEnvironment(e))' in environments,
      "environment read endpoint for recovery polling missing")

check('resource == "backups" && r.Method != http.MethodGet' in gateway and 'action = "approve"' in gateway,
      "backup mutations are not backups.approve protected")
check('"BACKUP_PRODUCTION_RESTORE_QUEUED"' in gateway,
      "production restore audit action missing")
check('strings.Contains(lowerKey, "/api/v1/central/administration?")' in gateway10,
      "Administration server cache invalidation missing")
check("String centralAdministrationInitialPath()" in frontend,
      "Administration canonical Central path helper missing")
check("add('/api/v1/central/administration')" in frontend,
      "Administration browser cache invalidation contract missing")
warm_start=frontend.find("void _warmControlPlane()")
warm_end=frontend.find("Future<void> _loadPublishedBrandAssets",warm_start)
warm=frontend[warm_start:warm_end] if warm_start>=0 and warm_end>warm_start else ""
check("centralAdministrationInitialPath()" not in warm and "api.prefetch(" not in warm,
      "Administration hard refresh must not launch browser prewarm/fan-out")

for token in [
    "Future<void> _restoreProduction(",
    "recoverability != 'VERIFIED'",
    "widget.productionRestoreEligible[partnerId] != true",
    "widget.canMutate",
    "scopeToPartnerIds: true",
]:
    check(token in backup_ui or token in admin_ui, f"recovery UI gate missing: {token}")

for token in [
    "  /api/v1/central/administration:",
    "  /api/v1/billing/company/documents:",
    "  /api/v1/backups/restore-points/{restorePointId}/restore:",
    "  /api/v1/backups/restores:",
    "  /api/v1/backups/restores/{restoreJobId}:",
    "    RestoreJob:",
]:
    check(token in openapi, f"OpenAPI CENTRAL-14 contract missing: {token}")

for token in [
    "'Administration Center': 'Adminisztrációs központ'",
    "'HIMATE Administration Center': 'HIMATE adminisztrációs központ'",
    "'Partner Administration Center': 'Partner adminisztrációs központ'",
    "'System Backup & Recovery': 'Rendszermentés és helyreállítás'",
]:
    check(token in localization, f"Hungarian CENTRAL-14 localization missing: {token}")

check("CENTRAL-14" in acceptance and "smoke_central_14.sh" in acceptance,
      "CENTRAL-14 acceptance document incomplete")

print("CENTRAL-14 static acceptance passed")
