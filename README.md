# HIMATE System - START-01 to START-03

First executable development package for the HIMATE central control platform.

## Included scope

- START-01: project skeleton and Render-ready application layout.
- START-02: Go authentication foundation and Flutter admin shell.
- START-03: responsive side navigation, mobile drawer and card-based dashboard.

Later START items are intentionally not implemented in this archive.

## Technology

- Backend: Go 1.23, standard library HTTP service.
- Frontend: Flutter Web.
- Target test hosting: Render web service named `himate`.
- Permanent database direction: PostgreSQL. Migration contracts are present; the START-01/03 runtime identity store is currently bootstrap-memory only.

## Local backend test

```sh
cd backend
go test ./...
```

## Local backend run

Set the required environment values first:

```sh
export HIMATE_SESSION_SECRET='replace-with-a-long-random-secret-value'
export HIMATE_BOOTSTRAP_ADMIN_NAME='Admin Name'
export HIMATE_BOOTSTRAP_ADMIN_EMAIL='admin@example.com'
export HIMATE_BOOTSTRAP_ADMIN_PASSWORD='replace-with-a-strong-password'
export PORT=10000
cd backend
go run ./cmd/api
```

The API is then available at `http://localhost:10000/api/v1/health`.

## Flutter development

A Flutter SDK is required:

```sh
cd frontend
flutter pub get
flutter run -d chrome
```

For a separate Flutter development port, set `ALLOWED_ORIGINS` on the backend to the exact Flutter origin. Production uses a same-origin single-container build.

## Render

The root `Dockerfile` builds the Flutter Web app and the Go service into one runtime image. `render.yaml` defines a web service named `himate` with `/api/v1/health` as the health check.

Before the first deployment set these secret values in Render:

- `HIMATE_BOOTSTRAP_ADMIN_NAME`
- `HIMATE_BOOTSTRAP_ADMIN_EMAIL`
- `HIMATE_BOOTSTRAP_ADMIN_PASSWORD`

`HIMATE_SESSION_SECRET` is configured for generated secret material in the Blueprint.

The final Render hostname depends on availability of the requested service name; the repository is configured to request `himate`.

## Important START-01/03 limitation

This round deliberately stops before the persistent PostgreSQL adapter. Bootstrap admins are recreated from environment configuration on process start. The included SQL migrations define the permanent data contract, but persistence wiring belongs to the next database integration step before any real operational data is entered.

## Security notes

- Session tokens are held in HttpOnly cookies, not browser local storage.
- Production Docker configuration enables `Secure` cookies.
- Passwords are hashed before they enter the runtime user store.
- The UI never displays synthetic partner/impact values.
- No secrets are committed to the repository.
