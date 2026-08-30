# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
go run ./cmd/api                          # Start the Fiber HTTP server
go test ./...                             # Run all tests
go test ./internal/user/usecase           # Run a single package's tests
go test -run TestService_Register_Success ./internal/user/usecase   # Run a single test
INTEGRATION=true go test ./test/integration   # Integration tests (skipped without this env var; needs a live DB)
make -C scripts build                     # go build ./...
make -C scripts fmt                       # gofmt -w over cmd/ config/ internal/ pkg/ test/
make -C scripts lint                      # golangci-lint, falls back to go vet
make -C scripts tidy                      # go mod tidy
docker compose up -d database             # PostgreSQL (default)
docker compose --profile mysql up -d mysql
docker compose --profile mongo up -d mongo
```

## Architecture

Go 1.22 / Fiber v2, **clean architecture** with dependency inversion. Module path `github.com/casper/go-fiber-clean-arch`.

### Request flow

Fiber global middleware (recovery, request logger, CORS) → route group `/api/v1` → per-route JWT guard → HTTP handler → use case service → repository interface → persistence adapter (SQL/Mongo)

### Layer structure per domain (`internal/user/`, `internal/auth/`)

| Layer | Path | Role |
|---|---|---|
| Entity | `internal/<domain>/entity/` | Pure domain types with invariants and factory functions |
| Repository | `internal/<domain>/repository/` | Interface + sentinel errors (no implementations) |
| Use Case | `internal/<domain>/usecase/` | Business logic; depends only on repository interfaces, validator, logger, and its own port interfaces (e.g. `usecase.OIDCProvider`) |
| Persistence | `internal/<domain>/infrastructure/persistence/` | SQL (`sqlx`) and MongoDB implementations of the repository interface |
| Delivery | `internal/<domain>/delivery/http/` | Fiber handlers, DTOs, route registration (`routes.go`) |

Sentinel errors are re-exported upward (`usecase.ErrEmailExists = repository.ErrEmailExists`) so handlers map them to HTTP status with `errors.Is`. Validation failures surface as `*validator.Error` (field → message map) and map to 422.

### The two domains

- **`internal/user`** — CRUD reference module. `/api/v1/users`, JWT-guarded when `APP_JWT_MIDDLEWARE_ENABLED=true`.
- **`internal/auth`** — OAuth2/OIDC login, refresh, and logout. `GET /api/v1/auth/login` (sets short-lived `oidc_state` / `oidc_nonce` / `oidc_verifier` HTTP-only cookies scoped to `/api/v1/auth`, redirects to the provider) and `GET /api/v1/auth/callback` (verifies state+nonce+PKCE, auto-provisions a `users` row + `auth_identities` row, returns a locally signed HS256 access token). When `APP_OIDC_ENABLED=false`, `DisabledHandler` registers the same two paths returning 503 — so route shape stays stable.

Auth depends on the user module's repository (cross-domain wiring lives in `internal/auth/delivery/http/routes.go`, not in the use case).

### Wiring

- `config/config.go` — Viper + godotenv, `APP_` prefix, validated in `AppConfig.Validate()` (fail-fast at startup).
- `internal/bootstrap/container.go` — `Container` (config, logger, DB connection, validator). Modules register routes via `RouteRegistrar` = `func(*fiber.App, *Container) error`.
- `cmd/api/main.go` — load config → build container → Fiber app + global error handler → `middleware.Register` → `container.RegisterRoutes(app, authhttp.Register, userhttp.Register)` → `/healthz` (always public) → listen, with signal-driven graceful shutdown.
- Each `routes.go` picks the repository implementation by `Config.Database.Driver` in a `buildRepository` switch, then mounts handlers under `/api/v1`. Copy this switch when adding a domain.

### JWT

Two halves, don't confuse them:
- `internal/auth/usecase/token.go` **issues** tokens (HS256, `sub`/`iss`/`aud`/`exp` + email/name/provider claims).
- `pkg/middleware.JWTProtected(cfg.JWT)` **verifies** them and stashes `auth.claims` / `auth.user_id` in `ctx.Locals`. It is applied per route group by the module, not globally — `/healthz` and the auth endpoints stay reachable without a token.

### Adding a new domain

Follow `docs/architecture/domain-checklist.md`: create `internal/<domain>/` mirroring the user module, expose a `Register` matching `RouteRegistrar`, and add it to the `container.RegisterRoutes(...)` call in `cmd/api/main.go`.

### Shared packages (`pkg/`, `internal/shared/`)

- `pkg/database` — Factory opening `sqlx` or Mongo per `APP_DATABASE_DRIVER`; returns one `Connection{SQL, Mongo}` where only the active field is non-nil
- `pkg/logger` — Zerolog wrapper; modules derive a child logger with `Str("module", …)`
- `pkg/middleware` — Recovery, request logger, CORS (global, config-toggled) + `JWTProtected` (per route); also stashes the logger in `ctx.Locals("logger")`
- `pkg/response` — JSON envelopes: `Success` → `{"data": …}`, `Error` → `{"error": {"message", "detail"}}`, `Paginated` → `{"data", "meta"}`. Use these instead of `ctx.JSON` so responses stay uniform.
- `internal/shared/validator` — go-playground wrapper returning `*validator.Error{Fields map[string]string}`

### SQL conventions

One SQL adapter serves both Postgres and MySQL: write reads/deletes with `?` placeholders and run them through `db.Rebind(query)`; write inserts/updates with `:named` params and `NamedExecContext`. Duplicate-key detection is string matching in `isDuplicateErr`.

### Configuration

Env-driven, `APP_` prefix. Viper keys with a dot map to an underscore: `HTTP.PORT` → `APP_HTTP_PORT`, `OIDC.ISSUER_URL` → `APP_OIDC_ISSUER_URL`. Middleware toggles are flat: `APP_REQUEST_LOGGER_ENABLED`, `APP_RECOVERY_ENABLED`, `APP_CORS_ENABLED`, `APP_JWT_MIDDLEWARE_ENABLED`. Drivers: `postgres`, `mysql`, `mongo`. `APP_JWT_SECRET` has no default and is required (and must be ≥32 chars when `APP_ENV=production`); mongo additionally requires `APP_MONGO_URI` and `APP_MONGO_DATABASE`; `APP_OIDC_ENABLED=true` requires issuer URL, client ID/secret, redirect URL, and `openid` in scopes. All defaults live in `config/config.go:setDefaults()`.

### Testing patterns

- Use case tests use hand-written stubs implementing `repository.Repository` (`internal/user/usecase/service_test.go`) and `usecase.OIDCProvider` (`fakeProvider` in `internal/auth/usecase/service_test.go`) — no mocking library
- `testify` for assertions; `zerolog.Nop()` for silent loggers
- Handler tests build a bare `fiber.New()` and drive it with `app.Test(httptest.NewRequest(...))`
- `test/integration/` boots the real container and Fiber app in-process, and disables JWT via `cfg.Middleware.JWT = false`

## Known gaps in the scaffold

These are unfinished parts of the starter, not bugs to be surprised by. `docs/boilerplate-status-report.md` is the longer handover version.

- **No `cmd/tools/migrate`.** `make -C scripts migrate-*` fails. Use the compose profile instead:
  `MIGRATION_CMD="migrate -path=/migrations -database postgres://app:app@database:5432/app?sslmode=disable up" docker compose --profile migrate run --rm migrator`
- **No `specs/`** despite README/AGENTS.md contribution steps referencing it.
- **No password login.** `usecase.Service.Register` stores a bcrypt hash that nothing verifies. Tokens come only from the OIDC flow, so either run Keycloak (below) or set `APP_JWT_MIDDLEWARE_ENABLED=false`.
- **No authorization rules** beyond "valid token" — a route sees `auth.user_id` and nothing else.
- **Access tokens are not revocable.** Refresh tokens rotate and can be revoked, but an issued access token stays valid until `exp`, so revocation latency equals `APP_JWT_ACCESS_TTL`. Every token now carries a `jti`, which is what a denylist would key on if one is ever added.
- **Expired and spent session rows are never pruned.** `auth_sessions` grows one row per refresh; used rows must be kept for replay detection. Add a periodic delete of rows past `expires_at` before this runs long in production.
- **Email-based account linking is unconditional.** `resolveProvisionedUser` attaches an OIDC login to any existing local user with the same verified email. Safe with one trusted issuer, but it becomes a takeover path once a second IdP or password login exists, because `POST /api/v1/users` accepts arbitrary emails. Marked with a `ponytail:` comment in `internal/auth/usecase/service.go`.
- **No FK from `auth_identities.user_id` to `users.id`.** Migration order puts identities first, and the `ALTER`/`DROP` syntax differs between Postgres and MySQL, which would break the one-adapter-serves-both convention. Orphan identities are possible.

## Sessions

`auth_sessions` holds one row per issued refresh token; rows sharing `session_id` are one
login's rotation family. Refresh tokens are opaque, stored only as a SHA-256 hash, single
use, and rotated on every refresh. Presenting a spent token revokes the entire family
(RFC 9700 replay detection) — `Service.Refresh` in `internal/auth/usecase/service.go`.

`ConsumeSession` is a conditional update (`WHERE used_at IS NULL AND revoked_at IS NULL`);
those predicates are the concurrency guards that stop two simultaneous refreshes from both
minting a pair, and stop a refresh that overlapped a logout. Keep it atomic in any new
adapter. `Refresh` then re-reads the predecessor after inserting the successor, because a
logout landing between the two writes would otherwise leave the successor unrevoked —
covered by `TestService_Refresh_LosesRaceAgainstConcurrentLogout`.

`POST /auth/logout` revokes the family AND returns `logout_url`. The client must navigate
to that URL — otherwise Keycloak's SSO cookie survives and the next login silently
re-authenticates the same user.

## Keycloak (local OIDC provider)

`docker compose --profile keycloak up -d keycloak` runs Keycloak 26 on port 8081 and imports `deploy/keycloak/realm-clean-arch.json`: realm `clean-arch`, confidential client `clean-arch-api`, PKCE S256 required, users `ada`/`ada` (verified) and `grace`/`grace` (unverified, must be rejected). Details and failure modes are in `docs/configuration.md`.

`KEYCLOAK_E2E=true go test ./test/integration -run TestKeycloak -v` drives the real browser flow end to end. It needs Postgres, Keycloak, migrations applied, and port 8080 free.

Two things that are easy to get wrong:

- `KC_HOSTNAME` and `APP_OIDC_ISSUER_URL` must match byte-for-byte; `iss` is built from the former and verified against the latter.
- Editing the realm file does nothing on restart — startup import skips an existing realm. Use `up -d --force-recreate keycloak`.
