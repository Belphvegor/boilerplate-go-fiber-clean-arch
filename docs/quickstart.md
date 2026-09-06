# Developer Quickstart

This guide walks through setting up the Clean Architecture Fiber Starter for local development.

## 1. Prerequisites

- Go 1.22 or newer in your `PATH`
- Docker + Docker Compose
- GNU Make (optional, for provided automation targets)

## 2. Configure Environment

```bash
cp .env.example .env
```

Update `.env` with secrets and database credentials. At minimum set:

- `APP_NAME`
- `APP_ENV`
- `APP_HTTP_PORT`
- `APP_DATABASE_DRIVER`
- Driver-specific DSN/URI values
- `APP_JWT_SECRET`

For the bundled local Keycloak, the OIDC values already match the imported realm.
Only enable it:

```env
APP_OIDC_ENABLED=true
APP_JWT_SECRET=replace-with-at-least-32-characters
```

## 3. Install Dependencies

```bash
go mod tidy
```

> **Note**: If you are running in a restricted environment where Go tooling cannot download modules, run the command outside the sandbox and commit the resulting `go.sum` file.

## 4. Start Supporting Services

```bash
docker compose --profile keycloak up -d database keycloak
```

- Omit `--profile keycloak` and `keycloak` when testing without OIDC.
- To use MySQL: `docker compose --profile mysql up -d mysql`
- To use MongoDB: `docker compose --profile mongo up -d mongo`

## 5. Run Database Migrations

For the default PostgreSQL database:

```bash
MIGRATION_CMD="migrate -path=/migrations -database postgres://app:app@database:5432/app?sslmode=disable up" \
  docker compose --profile migrate run --rm migrator
```

The migrator mounts `db/migrations`. Change the database URL when using another driver.
Migration `004` permanently removes local password hashes. For an existing deployment,
stop the old API before applying it and deploy the new binary immediately afterward.

MongoDB has no migration runner in this starter. Remove the legacy field once during
the same maintenance window:

```bash
mongosh "$APP_MONGO_URI/$APP_MONGO_DATABASE" \
  --eval 'db.users.updateMany({}, {$unset: {password_hash: ""}})'
```

## 6. Start the API

```bash
go run ./cmd/api
```

The server listens on `:${APP_HTTP_PORT}` and exposes `/healthz` for readiness checks. Misconfiguration results in a fail-fast error with remediation hints.

## 7. Smoke Test Keycloak Login

Open:

```text
http://localhost:8080/api/v1/auth/login
```

Sign in with the local fixture user:

```text
username: ada
password: ada
```

The callback returns a local access/refresh token pair. Use `access_token` on protected
routes:

```bash
curl http://localhost:8080/api/v1/users/<user-id> \
  -H "Authorization: Bearer <access-token>"
```

The callback's `user.id` is the local domain ID used by `GET /api/v1/users/{id}`.
The local profile is created automatically on the first successful login.

Public self-registration is disabled. To add a development user, open the Keycloak
Admin Console at <http://localhost:8081>, select realm `clean-arch`, create an enabled
user, mark the email verified, and set its credential there. The API never receives or
stores that password.

## 8. Run Tests

```bash
make -C scripts test
```

For integration suites:

```bash
docker compose up -d database
go test ./test/integration/...
```

The real Keycloak flow is opt-in because it requires both services:

```bash
KEYCLOAK_E2E=true go test ./test/integration -run TestKeycloak -count=1 -v
```

## 9. Tear Down

```bash
docker compose --profile keycloak down
```

If you started profile-specific services, include `--profile` flags when shutting them down.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `load configuration: config: APP_JWT_SECRET is required` | Set `APP_JWT_SECRET` in `.env`. |
| Database connection refused | Ensure the corresponding container is running and the DSN/URI matches credentials. |
| OIDC issuer mismatch | Keep `APP_OIDC_ISSUER_URL=http://localhost:8081/realms/clean-arch`. |
| Keycloak reports an invalid redirect URI | Keep the callback exactly `http://localhost:8080/api/v1/auth/callback`. |
| Realm changes do not appear | Recreate Keycloak with `docker compose --profile keycloak up -d --force-recreate keycloak`. |
| `cannot set capabilities` when running Go commands | Run Go tooling on a host with full Go toolchain access, then copy generated artifacts back. |

## Smoke Test Checklist

| Step | Expected Result | Status |
|------|-----------------|--------|
| `POST /api/v1/users` | `404 Not Found`; password signup does not exist | Pending |
| `GET /api/v1/users/{id}` | `200 OK` with user payload | Pending |
| `GET /healthz` | `200 OK` JSON `{ "status": "ok" }` | Pending |
| Keycloak login as `ada` | Callback returns tokens and auto-provisions the local profile | Pending |
| Keycloak login as `grace` | Callback returns `401` because email is unverified | Pending |

Happy building!
