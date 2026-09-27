#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
failures = []

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding='utf-8')

def check(ok: bool, message: str) -> None:
    if not ok:
        failures.append(message)

css = read('frontend/web/himate-brand-r4.css')
landing = read('frontend/web/landing.html')
frontend = read('frontend/lib/main.dart')
modules = read('frontend/lib/module_control_plane.dart')
gateway = read('services/cmd/gateway/central10.go')
gateway_main = read('services/cmd/gateway/main.go')
step3 = read('services/cmd/gateway/central_step3_snapshots.go')
step4 = read('services/cmd/gateway/central_step4_snapshots.go')
partner_snapshots = read('services/cmd/gateway/central_partner_workspace_snapshots.go')
fixture = read('services/cmd/partners/test_fixture.go')

# 1. Public landing geometry is restored to the Sep 19 reference while the
# protected current logo/brand asset remains untouched.
for token in [
    'CENTRAL-21: approved Sep 19 landing geometry restoration',
    '--public-inset:clamp(64px,5.1vw,92px);',
    'width:min(1510px,calc(100% - (var(--public-inset)*2)))',
    '.hero-bg{',
    'left:23%;',
    'right:-23%;',
    'padding-top:138px;',
    'min-height:clamp(100px,6.3vw,130px);',
    '--design-heading-font:"Cormorant Garamond";',
    '--design-body-font:"Inter";',
]:
    check(token in css, f'approved public layout/font contract missing: {token}')
check('/brand/himate_identity_wordmark_2026.webp' in landing,
      'protected HIMATE logo asset path changed')
restoration = css[css.find('CENTRAL-21: approved Sep 19 landing geometry restoration'):]
check('.brand-logo' not in restoration and 'identity_wordmark' not in restoration and 'himate_logo' not in restoration,
      'landing restoration layer must never override protected logo selectors/assets')
for page in ['landing.html','platform.html','modules.html','programs.html','impact.html','partners.html']:
    html = read('frontend/web/' + page)
    check('href="/himate-brand-r4.css"' in html, f'{page} no longer uses protected shared public stylesheet')

# 2-3. Critical Central reads are Last-Known-Good snapshots, not browser
# deadlines or persisted degraded models.
for token in [
    'a.warmMissingCentralSnapshots()',
    'a.warmMissingCentralPartnerWorkspaces()',
    'go a.runCentralStep3Materializer()',
    'go a.runCentralStep4Materializer()',
    'go a.runCentralPartnerWorkspaceMaterializer()',
]:
    check(token in gateway_main, f'gateway startup materializer contract missing: {token}')
for token in [
    'func centralSnapshotValid(key string, payload map[string]any) bool',
    'strings.EqualFold(central10String(payload["status"]), "healthy")',
    'central read-model refresh rejected; retaining last-known-good snapshot',
    'func (a *app) warmMissingCentralSnapshots()',
]:
    check(token in step3, f'Last-Known-Good snapshot contract missing: {token}')
for token in [
    'centralStep4PartnersKey',
    'centralStep4FinanceKey',
    'centralStep4AdministrationKey',
    'centralStep4SystemKey',
    'refreshCentralStep4Administration',
    'refreshCentralStep4System',
]:
    check(token in step4, f'authoritative Central screen materializer missing: {token}')

for forbidden in [
    '.timeout(const Duration(seconds: 3))',
    'TimeoutException(\'Partner module view timed out',
    'Finance snapshot is warming',
    'Module snapshot is warming',
]:
    check(forbidden not in frontend + '\n' + modules,
          f'forbidden CENTRAL-20 client failure UX survived: {forbidden}')

warm_start = frontend.find('void _warmControlPlane()')
warm_end = frontend.find('Future<void> _loadPublishedBrandAssets', warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ''
check('api.prefetch(' not in warm, 'hard-refresh control-plane prefetch storm survived')
check('Gateway owns authoritative read-model warming' in warm,
      'frontend no longer documents Gateway-owned warmup')

# 4 & 6. Partner deep link renders from a persistent per-partner LKG workspace;
# the browser request path does not orchestrate microservices.
for token in [
    'Future<Map<String, dynamic>> _loadPrimaryPartner()',
    "path: '/api/v1/central/partners'",
    "if ('${row['id'] ?? ''}' == partnerId)",
    "key: const Key('partner-workspace-loading-back')",
    "label: LText(uiLiteral('Back to Partners'))",
]:
    check(token in frontend, f'Partner workspace direct-load/back contract missing: {token}')

workspace_start = gateway.find('func (a *app) central10PartnerWorkspace(')
workspace_end = gateway.find('func central10NormalizeDashboardImpact', workspace_start)
workspace = gateway[workspace_start:workspace_end] if workspace_start >= 0 and workspace_end > workspace_start else ''
check('centralStep3SnapshotGet(key)' in workspace,
      'Partner workspace does not read the per-partner authoritative snapshot')
for forbidden in ['internalGET(', 'WaitGroup', 'context.WithTimeout(r.Context()', 'central10PartnerWorkspaceBudget']:
    check(forbidden not in workspace,
          f'Partner workspace request path still performs live orchestration: {forbidden}')
for token in [
    'centralPartnerWorkspacePrefix',
    'materializeCentralPartnerWorkspace',
    'refreshCentralPartnerWorkspaceSnapshots',
    'centralStep3Store(persistCtx, key, payload)',
]:
    check(token in partner_snapshots, f'per-partner LKG materializer missing: {token}')

# 5. Golden Test Partner represents six distinct historical months plus an active subscription.
for token in [
    "COUNT(DISTINCT date_trunc('month',i.service_period_start))",
    ") < 6",
    'billing.partner_plan_subscriptions',
    "'CUSTOM','MONTHLY','ACTIVE'",
    "'TEST_FIXTURE_STARTED'",
    "{6120, 31, 118, 24}",
    "{6840, 36, 132, 28}",
    "{7310, 39, 146, 31}",
    "{7890, 43, 158, 34}",
    "{8425, 47, 171, 38}",
    "{9180, 52, 189, 42}",
]:
    check(token in fixture, f'Golden Test Partner six-month contract missing: {token}')

# 7-8. Activation is persisted and immediately reflected in package eligibility.
for token in [
    "'availability': 'ACTIVE'",
    "'implementation_state': 'READY'",
    "'publication_status': 'PUBLISHED'",
]:
    check(token in modules, f'module activation persistence payload missing: {token}')
for token in [
    'applyCentralModuleMutationSnapshot',
    'central10PackageEligibleModule(module)',
    'strings.EqualFold(central10String(module["availability"]), "ACTIVE")',
    'strings.EqualFold(central10String(module["publication_status"]), "PUBLISHED")',
    'strings.EqualFold(central10String(module["implementation_state"]), "READY")',
]:
    check(token in gateway, f'module activation/package consistency contract missing: {token}')

if failures:
    print(f'CENTRAL-20 FAIL: {len(failures)} issue(s)')
    for failure in failures:
        print(' -', failure)
    sys.exit(1)

print('CENTRAL-20/21 LKG hard-refresh/public-layout/Test-Partner acceptance: PASS')
