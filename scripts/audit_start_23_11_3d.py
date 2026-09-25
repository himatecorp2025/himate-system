#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
portal_ui = (root / "frontend/lib/partner_portal.dart").read_text(encoding="utf-8")
partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
durability = (root / "services/cmd/gateway/phase2_durability.go").read_text(encoding="utf-8")
portal = (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

checks = [
    ("New Partner wizard captures an initial portal password", "final portalPassword = TextEditingController();" in frontend),
    ("New Partner wizard creates an owner portal identity through the durable saga",
     "await widget.api.post('/api/v1/partner-onboarding'" in frontend
     and "'portal_owner': {" in frontend
     and "func (a *app) ensureOnboardingOwner" in durability
     and "role_key" in durability
     and "'owner'" in durability),
    ("New Partner wizard no longer bypasses the durable saga with a direct owner POST",
     "await widget.api.post('/api/v1/partners/$partnerId/portal-users'" not in frontend),
    ("New Partner wizard uses the administrator email for portal access", "'email': contactEmail.text.trim()" in frontend),
    ("Partner Login has an explicit clear-password control", "tooltip: uiLiteral('Clear password')" in portal_ui),
    ("clear-password closes the browser autofill context", "TextInput.finishAutofillContext(shouldSave: false)" in portal_ui),
    ("partners cleanup is a forward migration", 'Version: 6, Name: "retire-fixed-manual-qa-partner"' in partners),
    ("identity cleanup is a forward migration", 'Version: 12,' in portal and 'Name:    "retire-fixed-manual-qa-partner-identity"' in portal),
    ("fixed partner startup seed is removed", "Persistent manual QA fixture. Keep until" not in partners),
    ("fixed portal startup constants are removed", "persistentTestPartnerPasswordHash" not in portal and "persistentTestPartnerID" not in portal),
    ("gateway uses the retirement identity migration", "retiredTestPartnerIdentityMigration()," in gateway),
    ("Partner Portal API uses precise access diagnostics", "if accessErr!=nil{writePartnerAccessError(w,accessErr);return}" in portal),
    ("release version", "version: 0.8.33-start-23.12" in openapi),
    ("render release version", "value: 0.8.33-start-23.12" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for label in failures:
        print("FAIL:", label)
    raise SystemExit(1)

print("START-23.11.3d partner onboarding static audit: PASS")
