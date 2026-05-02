# Boilerplate Status Report

This document is a handover report for engineers who need to understand, run, extend, or audit this Go Fiber clean architecture boilerplate.

Last updated: 2026-05-02

## Executive Summary

This repository is a Go 1.22 Fiber API boilerplate using clean architecture boundaries. It currently includes:

- A working HTTP API entrypoint in `cmd/api`.
- Environment-driven configuration in `config`.
- Shared bootstrap, logger, middleware, response, validator, and database helpers.
- A sample `user` bounded context with entity, use case, HTTP delivery, SQL persistence, and Mongo persistence.
- A generic OAuth2/OpenID Connect authentication bounded context in `internal/auth`.
- Route-level JWT protection for authenticated API endpoints.
- PostgreSQL, MySQL, and MongoDB repository support.
- Documentation for configuration, architecture, observability, quickstart, and OpenAPI.
- Makefile automation for build, test, format, lint, and migrations.

The boilerplate is suitable as a production-oriented starting point, but it is not a finished product. Teams still need to configure a real OIDC provider, add production-grade migrations tooling, tighten CORS, decide refresh-token/session policy, and add app-specific authorization rules.

## Current Architecture

The codebase follows a clean architecture style:

- `cmd/api`: application entrypoint, Fiber setup, middleware registration, route registration, graceful shutdown.
- `config`: environment loading, defaults, parsing, and fail-fast validation.
- `internal/bootstrap`: shared dependency container for configuration, logger, database, and validator.
- `internal/user`: sample domain module.
- `internal/auth`: OAuth2/OIDC authentication module.
- `pkg`: shared infrastructure packages used by multiple modules.
- `db`: migration and seed area.
- `docs`: human and API documentation.
- `test`: integration and end-to-end test area.
- `scripts`: Makefile automation.

The intended pattern for adding new features is:

1. Create a bounded context under `internal/<domain>`.
2. Keep business rules in `entity` and `usecase`.
3. Keep HTTP-specific logic under `delivery/http`.
4. Keep database code under `infrastructure/persistence`.
5. Define repository interfaces near the domain consumer.
6. Register routes through `internal/bootstrap.Container`.
7. Update tests, docs, and OpenAPI in the same branch.

## Authentication Status

The API now acts as a generic OAuth2/OIDC client, not as an OAuth authorization server.

Implemented flow:

1. `GET /api/v1/auth/login`
   - Generates state, nonce, and PKCE verifier.
   - Stores temporary values in HTTP-only cookies.
   - Redirects the user to the configured OIDC provider.

2. `GET /api/v1/auth/callback`
   - Validates callback state.
   - Exchanges authorization code with PKCE verifier.
   - Verifies the provider ID token through OIDC discovery/JWKS.
   - Validates nonce.
   - Resolves a local user.
   - Links the external provider identity by stable `(provider, subject)`.
   - Returns a short-lived local Bearer access token as JSON.

3. Protected routes
   - Local API JWTs are verified by route-level middleware.
   - `GET /api/v1/users/:id` is protected when `APP_JWT_MIDDLEWARE_ENABLED=true`.
   - `POST /api/v1/users`, `/healthz`, `/api/v1/auth/login`, and `/api/v1/auth/callback` remain public.

Important design choices:

- Authorization Code + PKCE is used.
- State and nonce are used for callback protection.
- Provider identity uses issuer/provider plus subject, not email alone.
- First successful OIDC login auto-provisions a local user if the provider returns a verified email.
- The API issues short-lived local Bearer access tokens.
- Refresh tokens are not implemented yet.
- Password registration remains available.

Core auth files:

- `internal/auth/usecase/service.go`: login completion and user provisioning.
- `internal/auth/usecase/token.go`: local JWT issuing.
- `internal/auth/infrastructure/oidc/provider.go`: production OIDC provider adapter.
- `internal/auth/infrastructure/persistence/identity_sql.go`: SQL identity mapping.
- `internal/auth/infrastructure/persistence/identity_mongo.go`: Mongo identity mapping.
- `pkg/middleware/http.go`: route-level JWT verification.

## How To Run Locally

1. Create local environment file:

```bash
cp .env.example .env
```

2. Set at minimum:

```env
APP_JWT_SECRET=replace-with-at-least-32-characters
APP_JWT_ISSUER=clean-arch-starter
APP_JWT_AUDIENCE=clean-arch-starter-api
```

3. Start the default database:

```bash
docker compose up -d database
```

4. Run the API:

```bash
go run ./cmd/api
```

5. Check health:

```bash
curl http://localhost:8080/healthz
```

## How To Enable OIDC

Register an OIDC client in a provider such as Keycloak, Auth0, Okta, Google, or another compliant provider.

Set:

```env
APP_OIDC_ENABLED=true
APP_OIDC_ISSUER_URL=https://issuer.example.com
APP_OIDC_CLIENT_ID=your-client-id
APP_OIDC_CLIENT_SECRET=your-client-secret
APP_OIDC_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback
APP_OIDC_SCOPES=openid profile email
APP_OIDC_LOGIN_STATE_TTL=5m
```

For production:

- Use HTTPS for `APP_OIDC_REDIRECT_URL`.
- Register the exact callback URL at the provider.
- Use a strong `APP_JWT_SECRET`; production rejects `change-me` and short secrets.
- Keep `APP_JWT_MIDDLEWARE_ENABLED=true`.
- Confirm the provider returns `email`, `email_verified`, `sub`, and `nonce`.

Login flow:

1. Browser opens `GET /api/v1/auth/login`.
2. API redirects to the OIDC provider.
3. Provider redirects back to `/api/v1/auth/callback`.
4. API returns:

```json
{
  "data": {
    "access_token": "eyJ...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "uuid",
      "name": "User Name",
      "email": "user@example.com"
    }
  }
}
```

Use the token:

```bash
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/users/<user_id>
```

## Configuration Overview

Configuration is environment-driven and loaded by `config.Load`.

Important groups:

- `APP_*`: application identity and runtime environment.
- `APP_HTTP_*`: server port and timeouts.
- `APP_DATABASE_*`: SQL database settings.
- `APP_MONGO_*`: MongoDB settings.
- `APP_JWT_*`: local API access token settings.
- `APP_OIDC_*`: external OIDC provider settings.
- `APP_LOGGING_*`: Zerolog output behavior.
- `APP_*_ENABLED`: middleware feature toggles.

The configuration layer fails fast when required values are missing or invalid. This is good for production because misconfigured deployments stop early instead of running in a broken state.

## Database Status

Supported database drivers:

- PostgreSQL
- MySQL
- MongoDB

The `user` module has SQL and Mongo repository implementations.

The `auth` module has SQL and Mongo identity repositories.

Auth identity storage:

- SQL table: `auth_identities`
- Mongo collection: `auth_identities`
- Unique identity key: `(provider, subject)`

Current migration files:

- `db/migrations/001_create_auth_identities.up.sql`
- `db/migrations/001_create_auth_identities.down.sql`

Important caveat: the migration area exists, and the Makefile has migration commands, but the actual `cmd/tools/migrate` command referenced by the Makefile is not present yet. A team should add or wire a real migration runner before relying on migrations in production deployments.

## Testing And Verification

Useful commands:

```bash
make -C scripts fmt
make -C scripts build
make -C scripts test
make -C scripts lint
```

Current test coverage includes:

- User use case tests.
- Auth service tests for auto-provisioning, invalid state, and unverified email.
- Auth HTTP handler tests for login redirect and disabled auth behavior.
- JWT middleware tests for valid and missing Bearer tokens.
- Config validation tests.
- Integration workflow test for user create/read, disabled unless `INTEGRATION=true`.

Last known verification commands that passed:

```bash
make -C scripts fmt
make -C scripts test
make -C scripts lint
make -C scripts build
```

## Strengths

The boilerplate has a clear modular architecture. Domain code is separated from HTTP and persistence code, so new engineers can follow one module and copy the pattern for another feature.

Configuration is explicit and validated early. This reduces runtime surprises and makes container or cloud deployments easier to reason about.

Authentication follows good baseline practices for an OIDC client:

- Authorization Code flow.
- PKCE.
- State.
- Nonce.
- OIDC discovery and JWKS validation.
- Stable provider-subject identity mapping.
- Short-lived local API tokens.

The code keeps the first version simple. It does not try to implement a full OAuth authorization server, refresh token rotation, account management UI, organization membership, or policy engine before the app needs them.

The repository supports multiple persistence backends. This is useful for a boilerplate because teams can start with PostgreSQL, MySQL, or MongoDB without rewriting the domain layer.

The docs are already organized around real engineer workflows: quickstart, configuration, architecture, observability, OpenAPI, and this status report.

## Current Limitations And Risks

Refresh tokens are not implemented. Users must re-authenticate when the local access token expires. This is simple and safer for v1, but not ideal for long-lived browser or mobile sessions.

Authorization is minimal. The API authenticates users but does not yet enforce ownership checks, roles, permissions, organizations, or scopes. For example, a valid token can reach the protected user-read route; the handler does not yet verify that the requester may read that specific user.

Migration tooling is incomplete. Migration files exist, but the Makefile references `go run ./cmd/tools/migrate`, and that command is not currently in the repository.

CORS uses Fiber's default permissive middleware when enabled. Production deployments should configure allowed origins, methods, and headers explicitly.

OIDC provider testing is mostly mocked. The auth use case and handlers are tested, but there is no full local Keycloak/Auth0-compatible integration test yet.

The SQL migration is intentionally generic. Teams may need database-specific improvements such as PostgreSQL UUID types, foreign keys to `users(id)`, index naming, and timestamp precision.

Mongo indexes are created lazily by the auth repository. This is convenient for development, but production teams may prefer index creation as an explicit deployment step.

The password registration path still exists alongside OIDC. That is useful for flexibility, but a real product should decide whether password login is supported, disabled, or moved behind a feature flag.

No rate limiting is currently applied to auth endpoints. Public endpoints such as login, callback, and registration should have rate limits before internet exposure.

No CSRF policy is needed for Bearer-only API usage, but if the project later moves tokens into cookies, CSRF protection must be added.

Secrets are environment-based only. This is acceptable for a boilerplate, but production should use a secret manager or platform secret injection.

Observability is basic. Request logs and Zerolog are present, but there are no distributed traces, metrics export, auth audit events, or alerting hooks yet.

## Recommended Next Steps

For production readiness:

1. Add a real migration runner under `cmd/tools/migrate` or replace Makefile migration commands with the team's migration tool.
2. Add provider-specific local development documentation, preferably with Keycloak in Docker Compose.
3. Add full OIDC integration tests against a local provider.
4. Add authorization checks for protected domain actions.
5. Configure strict CORS for production.
6. Add rate limiting for public auth and registration endpoints.
7. Decide refresh-token strategy:
   - keep access-token-only for simple internal tools,
   - add rotating refresh tokens for browser/mobile apps,
   - or integrate with provider session renewal.
8. Add structured audit logs for login success, login failure, identity link, and suspicious callback failures.
9. Add SQL foreign keys and database-specific migration variants if the production database is known.
10. Update `README.md` to link this report and the auth flow once the team has settled the final provider.

## Junior Engineer Guide

If you are new to this boilerplate, start here:

1. Read `README.md`.
2. Read `docs/quickstart.md`.
3. Read `docs/configuration.md`.
4. Read `docs/architecture/overview.md`.
5. Study `internal/user` as the simplest domain example.
6. Study `internal/auth` as the authentication example.
7. Run:

```bash
make -C scripts test
```

8. Add new business features by copying the `internal/user` layering style, not by putting everything into handlers.

When changing authentication, be careful with:

- ID token validation.
- State and nonce handling.
- PKCE verifier/challenge handling.
- Provider identity mapping.
- JWT issuer, audience, and expiry validation.
- Avoiding email-only account linking.

When changing persistence, update:

- Repository interface.
- SQL adapter.
- Mongo adapter.
- Migration files.
- Tests.
- Docs.

## Final Assessment

This boilerplate is in a healthy state for a starter project. It has a clean structure, practical defaults, a working example domain, and a real OIDC authentication foundation. The most important remaining work is not to add more abstraction, but to complete deployment-level production concerns: migrations, provider integration testing, authorization policy, CORS, rate limiting, and operational observability.
