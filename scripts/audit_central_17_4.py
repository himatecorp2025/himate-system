#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors = []

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        errors.append(f"missing file: {path}")
        return ""
    return p.read_text(encoding="utf-8")

def check(condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)

main = read("frontend/lib/main.dart")
cms = read("frontend/lib/cms_page.dart")
admin = read("frontend/lib/administration_center.dart")
modules = read("frontend/lib/module_control_plane.dart")
round1 = read("frontend/lib/central17_round1.dart")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
step3 = read("services/cmd/gateway/central_step3_snapshots.go")

# Typography: admin control-plane serif = Lora, functional UI = Inter.
check("GoogleFonts.interTextTheme" in main, "Inter global text theme missing")
check("GoogleFonts.loraTextTheme" in main, "Lora serif text theme missing")
for name, source in {
    "main.dart": main,
    "cms_page.dart": cms,
    "administration_center.dart": admin,
    "module_control_plane.dart": modules,
    "central17_round1.dart": round1,
}.items():
    check("GoogleFonts.cormorantGaramond" not in source, f"legacy Cormorant survived in {name}")

# Impact: degraded refreshes are never allowed to overwrite a healthy screen
# snapshot. Existing LKG data remains visible while materialization retries.
check('centralStep3Store(persistCtx, centralStep4ImpactKey, payload)' in step4,
      "Impact materializer is not routed through the authoritative snapshot store")
check('strings.EqualFold(central10String(payload["status"]), "healthy")' in step3,
      "Impact degraded refresh can bypass the Last-Known-Good gate")
check('central read-model refresh rejected; retaining last-known-good snapshot' in step3,
      "Impact failed refresh does not retain Last-Known-Good data")
check("Impact data is loading" in main and "Impact snapshot is warming" in main,
      "Impact non-blocking first-snapshot UI missing")
check("Loading the latest impact and evidence snapshot." not in main,
      "Impact full-page blocking loader survived")

# Website: aggregate read failure must not blank the workspace.
check("DIRECT_FALLBACK" in cms, "Website direct fallback read model missing")
check("/api/v1/cms/pages" in cms and "/api/v1/cms/media" in cms and "/api/v1/environments" in cms,
      "Website fallback sources incomplete")
check("Website data is loading" in cms and "Website data is partially unavailable" in cms,
      "Website non-blocking state UI missing")
check("child: _BrandLoading()," not in cms[cms.find("Widget build(BuildContext context)"):cms.find("class _WebsiteHubCard")],
      "Website overview still replaces the full workspace with a loader")

# System: structure remains visible before/without the central snapshot.
check("final provisional = snapshot.data == null;" in main,
      "System provisional render state missing")
check("No cached operations data yet" not in main,
      "System full-page empty-state blocker survived")

# Reference hierarchy cleanup.
check("NewPartnerCard(onTap: addPartner)" not in main,
      "Duplicate New Partner tile survived the reference grid")
check("class _PackageFeatureSummary" in main and "class _PackageBenefitLine" in main,
      "Reference package-card hierarchy missing")
check("_Central17TrendChart(partnerValues:modules,moneyValues:partners,labels:labels)" in round1,
      "Module empty chart scaffold missing")

if errors:
    print(f"FAIL: CENTRAL-17.4 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-17.4 runtime resilience, Lora/Inter typography and reference fidelity: PASS")
