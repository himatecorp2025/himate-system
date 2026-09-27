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
    "localCtx,localCancel:=context.WithTimeout",
    "QueryRowContext(localCtx,`SELECT COUNT(*) FROM identity.users WHERE active=TRUE`)",
    'unavailable=append(unavailable,"administrators")',
    'if status=="healthy" {',
]:
    check(token in admin, f"Administration timeout/cache contract missing: {token}")

for token in [
    "_cacheableGetResponse",
    "data['ready'] == false",
    "onRefresh != null && _cacheableGetResponse(path, freshData)",
    "Future<void> load({bool force = false, bool quiet = false})",
    "unawaited(load(force: true, quiet: true))",
]:
    check(token in frontend, f"frontend refresh-state contract missing: {token}")

if failures:
    print(f"CENTRAL-19 FAIL: {len(failures)} issue(s)")
    for failure in failures:
        print(" -", failure)
    sys.exit(1)

print("CENTRAL-19 refresh consistency static acceptance: PASS")
