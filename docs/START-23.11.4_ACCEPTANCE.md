# START-23.11.4 Acceptance — Partner Workspace & Personalization

## Purpose

START-23.11.4 lets each partner adapt the visual identity and terminology of its own HIMATE workspace without changing HIMATE application mechanics.

**Presentation may change. Canonical system meaning may not.**

## Workspace identity

A partner with `design.write` may configure:
- a workspace/company display name;
- an own-tenant workspace logo selected from partner-owned design media;
- primary, sidebar, background, accent and text colors;
- one default module preference.

The workspace name is a tenant label. It does not rename HIMATE. The Partner Portal retains a visible **Powered by HIMATE** system identity.

The workspace logo is tenant isolated. A partner cannot reference another tenant's media.

## Contrast protection

Brand colors use `#RRGGBB`.

Text/background combinations must maintain at least 4.5:1 contrast. The Portal also derives a readable foreground for the configured sidebar color, so a light or dark sidebar does not silently make navigation unreadable.

## Module presentation

Every Marketplace card retains the canonical HIMATE module key and official name.

A partner administrator may set:
- custom display name;
- short workspace description;
- an icon from the HIMATE icon library, or one partner-owned custom icon;
- card color.

The card editor always exposes the official HIMATE name and canonical key.

**Reset to HIMATE default** deletes only the presentation override.

Customization must never modify:
- route;
- API;
- permission key;
- billing;
- workflow;
- canonical module key;
- entitlement or access state.

## Default module

The default module is a presentation/navigation preference.

The Gateway validates it against the authenticated tenant's real Marketplace state. Only an `ACTIVE` and executable module may be selected.

The current Partner Portal does not invent a module runtime route. On initial workspace load, the Portal opens the Modules surface and moves the selected default module card to the first position. This preserves the canonical routing contract while giving the tenant a useful daily landing preference.

## Persistence and isolation

Workspace identity persists in `cms.partner_workspace_settings`.

Per-module presentation persists in `cms.partner_module_presentations`.

Both are keyed by partner tenant. Partner-owned logos/icons are validated against `cms.media_assets.owner_type='PARTNER'` and matching `owner_id`.

Workspace and module-presentation mutations are audited.

## Existing Design Profiles

START-23.8 tenant Design Profiles remain supported. START-23.11.4 adds workspace identity and module presentation as explicit partner-facing layers instead of replacing the existing visual-theme system.

## Out of scope

- user-to-module permissions: START-23.11.5;
- central notification engine: START-23.11.6;
- Partner Portal closure audit: START-23.11.7;
- new module runtime routes or changes to canonical module behavior;
- a new workspace-specific favicon requirement. Existing Design Profile favicon support remains available.

## Release

Release contract: `0.8.29-start-23.11.4`.

Automated evidence:
- static/source contract: `python3 scripts/audit_start_23_11_4.py`;
- Compose E2E: `sh scripts/smoke_start_23_11_4.sh http://127.0.0.1:8080`;
- inherited START-23.11.3 marketplace/commercial/onboarding acceptance remains mandatory.
