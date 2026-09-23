# START-23.11.3a Acceptance — Platform Secrets & Provider Readiness

## Scope

START-23.11.3a adds a write-only HIMATE platform-secret vault and changes optional external-provider configuration from a process-start blocker into an explicit capability state.

The control plane must remain deployable and testable before Stripe or Render credentials are supplied. Missing credentials may disable only the dependent provider operation. They must never silently enable provider actions or fall back to insecure credentials.

## Secret vault contract

The Gateway owns the administrator-facing secret-management surface.

Supported keys are explicitly allowlisted:
- `stripe_secret_key` / `STRIPE_SECRET_KEY`;
- `stripe_webhook_secret` / `STRIPE_WEBHOOK_SECRET`;
- `render_api_key` / `RENDER_API_KEY`.

Values are encrypted with AES-256-GCM before persistence in `identity.platform_secrets`.

The API is write-only:
- GET returns metadata and configuration state only;
- PUT accepts a secret but never returns it;
- DELETE removes the stored value;
- raw secret values are never exposed to Flutter;
- audit request/response snapshots redact secret-bearing fields.

Only the HIMATE system owner may create, replace or remove platform secrets. Other administrators with Administration read access may see configuration status only.

## Provider readiness contract

Payments may start in production `stripe` mode without Stripe credentials.

When credentials are absent:
- the Payments health endpoint stays healthy;
- `provider_configured=false`;
- `configuration_required=true`;
- charge creation fails closed with `503 PAYMENT_PROVIDER_UNCONFIGURED`;
- Stripe webhook processing fails closed with the same configuration-required boundary.

When credentials are supplied later through HIMATE Administration, Payments resolves them dynamically from the encrypted vault. Process-level environment variables remain valid higher-priority overrides.

Runtime follows the same lookup model for `RENDER_API_KEY`: environment configuration takes priority, otherwise the encrypted vault is used.

## UI contract

Administration contains **Secrets & API Keys**.

Each supported credential shows:
- provider;
- consuming HIMATE service;
- environment-key name;
- CONFIGURED or CONFIGURATION REQUIRED status.

The raw stored value is never displayed. Replacement requires entering a new value. Removal requires confirmation.

## Security constraints

- no real provider secret is committed to Git;
- arbitrary environment-variable names cannot be created through this endpoint;
- raw secret values are absent from GET responses;
- raw secret values are redacted from audit state;
- provider operations remain fail-closed when required credentials are missing;
- tenant/module authorization is unaffected.

## Release

Release contract: `0.8.18-start-23.11.3a`.

This is a deployment-readiness patch after START-23.11.3 and before START-23.11.4 Partner Workspace & Personalization.
