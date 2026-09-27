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
frontend = read('frontend/lib/main.dart')
modules = read('frontend/lib/module_control_plane.dart')
gateway = read('services/cmd/gateway/central10.go')
gateway_main = read('services/cmd/gateway/main.go')
step3 = read('services/cmd/gateway/central_step3_snapshots.go')
step4 = read('services/cmd/gateway/central_step4_snapshots.go')
fixture = read('services/cmd/partners/test_fixture.go')

# 1. Public landing/subpage geometry is restored without undoing approved fonts.
for token in [
    '--public-inset:24px;',
    '.wrap{width:min(1540px,calc(100% - 48px));margin-inline:auto}',
    'width:min(1540px,calc(100% - 48px))!important;',
    '--design-heading-font:"Cormorant Garamond";',
    '--design-body-font:"Inter";',
]:
    check(token in css, f'public layout/font contract missing: {token}')
for page in ['landing.html','platform.html','modules.html','programs.html','impact.html','partners.html']:
    html = read('frontend/web/' + page)
    check('href="/himate-brand-r4.css"' in html, f'{page} no longer uses protected shared public stylesheet')

# 2-3. Critical Central routes must be renderable on cold hard refresh.
check('a.warmMissingCentralSnapshots()' in gateway_main, 'gateway becomes live before critical Central snapshots are warmed')
for token in [
    'func (a *app) warmMissingCentralSnapshots()',
    'centralStep3RegistryKey, a.refreshCentralStep3Registry',
    'centralStep3PlansKey, a.refreshCentralStep3Plans',
    'centralStep4PartnersKey, a.refreshCentralStep4Partners',
    'centralStep4FinanceKey, a.refreshCentralStep4Finance',
    '"status": "unavailable"',
]:
    check(token in step3 or token in step4, f'cold snapshot contract missing: {token}')
check('explicit unavailable model' in step4, 'Partners first-run failure can still leave no renderable snapshot')
for token in [
    ".timeout(const Duration(seconds: 3))",
    "class _PartnersPageState",
    "class _PackagesPageState",
    "class _FinancePageState",
]:
    check(token in frontend, f'bounded Central frontend request contract missing: {token}')
check(modules.count('.timeout(const Duration(seconds: 3))') >= 2, 'Module registry/commercial cold loads are not both bounded')

# 4 & 6. Partner deep link renders from hot portfolio first and always provides back navigation.
for token in [
    'Future<Map<String, dynamic>> _loadPrimaryPartner()',
    "path: '/api/v1/central/partners'",
    "if ('${row['id'] ?? ''}' == partnerId)",
    "key: const Key('partner-workspace-loading-back')",
    "label: LText(uiLiteral('Back to Partners'))",
]:
    check(token in frontend, f'Partner workspace direct-load/back contract missing: {token}')
for token in [
    'central10PartnerWorkspaceBudget = 1500 * time.Millisecond',
    'centralStep3SnapshotGet(centralStep4PartnersKey)',
    'if livePartner != nil',
    'if status == "healthy"',
]:
    check(token in gateway, f'Partner workspace backend fallback contract missing: {token}')

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

print('CENTRAL-20 hard-refresh/public-layout/Test-Partner acceptance: PASS')
