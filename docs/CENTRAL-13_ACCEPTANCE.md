# CENTRAL-13 — Website, Marketing & Partner Operations

## Objective

CENTRAL-13 turns the existing Website & Marketing surface into a structured, backend-first operations workspace without introducing an unnecessary new microservice.

## Architecture

- CMS remains authoritative for website content, media, SEO and brand design.
- Connector remains authoritative for partner connector credentials, runtime sync state and Website Adapters.
- Partners remains authoritative for partner identity and lifecycle.
- Gateway composes the partner-first Connections read model and preserves RBAC.
- Flutter is presentation-only: it renders backend-composed partner connection rows and uses existing CMS actions.

## Website & Brand

- Website & Marketing becomes a card-based hub with focused sub-workspaces.
- Design Guide color fields are named:
  - Primary Color
  - Brand Color
  - Page Background
  - Body Text Color
- Every color field exposes a visible swatch / picker and synchronized HEX value.
- Heading and body font catalogs are expanded independently.
- Layout families remain presentation-only; they never alter CMS content or business logic.
- The real website renderer applies `classic_editorial`, `modern_grid` or `minimal` CSS.
- Design preview uses the existing real composed preview at:
  - Desktop 1440 × 900
  - Tablet 834 × 1194
  - Mobile 390 × 844
- The UI surfaces the truthful workflow:
  - Draft
  - Preview
  - Publish
  - Active Brand
- Active Brand publish is backed by the existing CMS published design snapshot and audit trail.

## Marketing

- SEO & Keywords and Customer Inbox are separate focused workspaces.
- Hungarian labels and the new explanatory descriptions are localized.
- CMS, Contact and Connector cards are permission-aware.

## Partner Data Connections

The main view is partner-first. It is not organized around Klaviyo or any other vendor.

Each partner card exposes:

- Partner Name
- Connection Status: `ACTIVE`, `INACTIVE`, `SUSPENDED`, `DELETED`
- Last Successful Sync
- Last Error
- Connection Type
- Integrations count

Runtime integrations come only from real Connector credentials/state and Website Adapter records. A vendor-specific type can appear only inside a partner's `integrations[]` when such a real record exists.

The Gateway endpoint is:

`GET /api/v1/central/connections`

It joins the authoritative partner registry with Connector runtime data, is protected by `connectors.read`, supports query/status filtering and is prewarmed with exact cache-key parity.

## Acceptance

Static:

`python3 scripts/audit_central_13.py`

Containerized runtime:

`sh scripts/smoke_central_13.sh http://127.0.0.1:8080`

Merge gate remains the full HIMATE CI: Go, Flutter, Compose, all historical START/CENTRAL acceptance steps and the CENTRAL-13 acceptance must be green.
