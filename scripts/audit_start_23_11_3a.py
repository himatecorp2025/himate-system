#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

def text(path):
    return (root / path).read_text(encoding="utf-8")

checks = {
    "gateway secret API": ("services/cmd/gateway/platform_secrets.go", "func (a *app) adminSecrets"),
    "secret allowlist": ("services/cmd/gateway/platform_secrets.go", "stripe_webhook_secret"),
    "encrypted storage": ("services/internal/common/common.go", "EncryptPlatformSecret"),
    "AES GCM": ("services/internal/common/common.go", "cipher.NewGCM"),
    "payments graceful readiness": ("services/cmd/payments/main.go", "PAYMENT_PROVIDER_UNCONFIGURED"),
    "payments vault lookup": ("services/cmd/payments/main.go", "LoadPlatformSecret"),
    "runtime vault lookup": ("services/cmd/runtime/main.go", "render_api_key"),
    "Flutter secrets panel": ("frontend/lib/secrets_admin.dart", "Secrets & API Keys"),
    "system-owner mutation guard": ("services/cmd/gateway/platform_secrets.go", "if !u.SystemOwner"),
    "release version": ("docs/openapi.yaml", "0.8.31-start-23.11.6"),
}

failures = []
for label, (path, needle) in checks.items():
    if needle not in text(path):
        failures.append(f"{label}: {needle!r} missing from {path}")

render = text("render.yaml")
if "value: 0.8.31-start-23.11.6" not in render:
    failures.append("render.yaml release version is not aligned")
if "sync: false" not in render or "STRIPE_SECRET_KEY" not in render:
    failures.append("runtime-only Stripe environment compatibility was removed")

if failures:
    for failure in failures:
        print("FAIL:", failure)
    raise SystemExit(1)

print("START-23.11.3a static audit: PASS")
