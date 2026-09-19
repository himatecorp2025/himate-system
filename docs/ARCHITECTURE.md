# HIMATE START-01 to START-03 Architecture

## Runtime shape

The first development round is a single Render web service. The Docker build compiles Flutter Web, compiles the Go API, and places the Flutter output beside the Go binary. The Go server serves both `/api/v1/*` and the Flutter single-page application.

This gives the test environment one public service name and one same-origin security boundary. The requested target service name is `himate`, which is intended to become `https://himate.onrender.com` if that Render hostname is available to the account.

## Backend

The Go backend currently owns:

- configuration validation;
- bootstrap administrator creation;
- password hashing;
- signed HttpOnly sessions;
- authentication endpoints;
- authenticated dashboard summary;
- HTTP security headers;
- CORS policy for local development;
- static Flutter Web delivery;
- health endpoint.

No partner, licensing, provisioning or impact business logic is implemented in this round.

## Frontend

Flutter Web currently owns:

- login UI;
- responsive authenticated shell;
- desktop navigation rail;
- mobile/tablet drawer;
- dashboard KPI cards;
- future-section navigation placeholders.

The dashboard starts with verified zero values. It does not contain demonstration figures that could later be confused with actual partner impact data.

## Data strategy

A PostgreSQL migration contract exists from the first round so identifiers and auditability can grow in the intended direction. The executable bootstrap store remains memory-based until the permanent database adapter is implemented. Therefore START-01/03 is suitable as a tested application foundation, not yet as a persistent production administration system.

## Evidence discipline

Future impact metrics must carry source, period, actor and evidence metadata. Marketing copy should focus on HIMATE's arts-technology mission and measurable partner outcomes. The platform must never manufacture impact values; uncollected values stay explicitly empty or zero until a verifiable source exists.
