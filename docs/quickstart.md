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

## 3. Install Dependencies

```bash
go mod tidy
```

> **Note**: If you are running in a restricted environment where Go tooling cannot download modules, run the command outside the sandbox and commit the resulting `go.sum` file.

## 4. Start Supporting Services

```bash
docker compose up -d database
```

- To use MySQL: `docker compose --profile mysql up -d mysql`
- To use MongoDB: `docker compose --profile mongo up -d mongo`

## 5. Run Database Migrations

Use your preferred migration tool. Example with `migrate` container profile:

```bash
docker compose --profile migrate run --rm migrator
```

Mount migration files from `db/migrations` and pass the desired command through `MIGRATION_CMD`.

## 6. Start the API

```bash
go run ./cmd/api
```

The server listens on `:${APP_HTTP_PORT}` and exposes `/healthz` for readiness checks. Misconfiguration results in a fail-fast error with remediation hints.

## 7. Smoke Test the User API

```bash
curl -X POST http://localhost:${APP_HTTP_PORT}/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"Str0ngPass!"}'
```

Use the returned `id` when calling the `GET /api/v1/users/{id}` endpoint (requires JWT when enabled).

## 8. Run Tests

```bash
make -C scripts test
```

For integration suites:

```bash
docker compose up -d database
go test ./test/integration/...
```

## 9. Tear Down

```bash
docker compose down
```

If you started profile-specific services, include `--profile` flags when shutting them down.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `load configuration: config: APP_JWT_SECRET is required` | Set `APP_JWT_SECRET` in `.env`. |
| Database connection refused | Ensure the corresponding container is running and the DSN/URI matches credentials. |
| `cannot set capabilities` when running Go commands | Run Go tooling on a host with full Go toolchain access, then copy generated artifacts back. |

## Smoke Test Checklist

| Step | Expected Result | Status |
|------|-----------------|--------|
| `POST /api/v1/users` with sample payload | `201 Created` and response body with user ID | Pending (execute after local setup) |
| `GET /api/v1/users/{id}` | `200 OK` with user payload | Pending |
| `GET /healthz` | `200 OK` JSON `{ "status": "ok" }` | Pending |

> **Note**: Smoke tests could not be executed inside the sandbox. Run them locally after completing the setup steps to confirm end-to-end behaviour.

Happy building!
