#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
failures = []

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")

def check(ok: bool, message: str) -> None:
    if not ok:
        failures.append(message)

gateway = read("services/cmd/gateway/central10.go")
gateway_main = read("services/cmd/gateway/main.go")
admin = read("services/cmd/gateway/central14.go")
snapshots = read("services/cmd/gateway/central_step3_snapshots.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
frontend = read("frontend/lib/main.dart")

for token in [
    'out["module_limit"] = 10',
    'out["module_limit"] = 20',
    'out["selection_mode"] = "UNLIMITED"',
    "central10PackageEligibleModule(module)",
    "central10MergeModuleSnapshot",
    "applyCentralModuleMutationSnapshot",
]:
    check(token in gateway, f"gateway refresh/package contract missing: {token}")

check("a.applyCentralModuleMutationSnapshot(r.URL.Path, state)" in gateway_main,
      "successful module mutation is not reconciled into the hot registry snapshot")

for token in [
    "func (a *app) materializeCentralAdministration(ctx context.Context)",
    "centralSnapshotForRead(r.Context(), centralStep4AdministrationKey)",
    'centralStep4Meta(started, centralStep4AdministrationKey, updatedAt, "healthy", []string{})',
]:
    check(token in admin, f"Administration authoritative snapshot contract missing: {token}")

for token in [
    "func centralSnapshotValid(key string, payload map[string]any) bool",
    'strings.EqualFold(central10String(payload["status"]), "healthy")',
    "central read-model refresh rejected; retaining last-known-good snapshot",
]:
    check(token in snapshots, f"Last-Known-Good persistence contract missing: {token}")
check("centralStep4AdministrationKey" in step4 and "refreshCentralStep4Administration" in step4,
      "Administration background materializer is not registered")

for token in [
    "_cacheableGetResponse",
    "data['ready'] == false",
    "onRefresh != null && _cacheableGetResponse(path, freshData)",
    "void _warmControlPlane()",
    "Gateway owns authoritative read-model warming",
]:
    check(token in frontend, f"frontend authoritative refresh-state contract missing: {token}")
warm_start = frontend.find("void _warmControlPlane()")
warm_end = frontend.find("Future<void> _loadPublishedBrandAssets", warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ""
check("api.prefetch(" not in warm, "frontend still launches a hard-refresh prefetch storm")
check("unawaited(load(force: true, quiet: true))" not in frontend,
      "Finance still contains CENTRAL-19 retry polling")

if failures:
    print(f"CENTRAL-19 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-19 refresh consistency static acceptance: PASS")
