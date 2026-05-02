# Configuration Matrix

The starter reads configuration exclusively from environment variables (loadable via `.env` in local environments). This matrix summarises available options and default values.

| Variable | Description | Default | Notes |
|----------|-------------|---------|-------|
| `APP_NAME` | Service identifier used in logs and metrics. | `clean-arch-starter` | Displayed in log entries and Fiber banner. |
| `APP_ENV` | Runtime environment (`development`, `staging`, `production`). | `development` | Toggle middleware defaults and logging verbosity. |
| `APP_HTTP_PORT` | HTTP listening port. | `8080` | Used by Fiber `Listen`. |
| `APP_HTTP_READ_TIMEOUT` | Max time to read request body. | `10s` | Parsed as Go duration string. |
| `APP_HTTP_WRITE_TIMEOUT` | Max time to write response. | `10s` | Parsed as Go duration string. |
| `APP_DATABASE_DRIVER` | Database backend (`postgres`, `mysql`, `mongo`). | `postgres` | Controls which repository implementation is resolved. |
| `APP_DATABASE_DSN` | SQL DSN (Postgres/MySQL). | `postgres://app:app@localhost:5432/app?sslmode=disable` | Required when driver is `postgres` or `mysql`. |
| `APP_DATABASE_MAX_OPEN_CONNS` | Max open SQL connections. | `10` | Tune per workload. |
| `APP_DATABASE_MAX_IDLE_CONNS` | Max idle SQL connections. | `5` | |
| `APP_DATABASE_CONN_MAX_LIFE` | Max lifetime for SQL connections. | `30m` | |
| `APP_DATABASE_METRICS_ENABLED` | Toggle database metrics instrumentation. | `true` | Placeholder for future metrics exports. |
| `APP_MONGO_URI` | MongoDB connection URI. | `mongodb://app:app@localhost:27017` | Required when driver is `mongo`. |
| `APP_MONGO_DATABASE` | MongoDB database name. | `app` | |
| `APP_JWT_SECRET` | HMAC secret for JWT middleware. | _none_ | Must be set for any environment. |
| `APP_JWT_ISSUER` | JWT issuer claim. | `clean-arch-starter` | |
| `APP_JWT_AUDIENCE` | JWT audience claim for this API. | `clean-arch-starter-api` | Protected routes reject tokens for other audiences. |
| `APP_JWT_ACCESS_TTL` | Access token lifetime. | `15m` | Parsed as Go duration. |
| `APP_OIDC_ENABLED` | Enable OAuth2/OIDC login endpoints. | `false` | When `true`, OIDC client settings are required. |
| `APP_OIDC_ISSUER_URL` | External OIDC issuer URL. | _none_ | Used for discovery and ID token issuer validation. |
| `APP_OIDC_CLIENT_ID` | OIDC client ID. | _none_ | Must match provider registration. |
| `APP_OIDC_CLIENT_SECRET` | OIDC client secret. | _none_ | Keep secret; never commit real values. |
| `APP_OIDC_REDIRECT_URL` | Callback URL registered at the provider. | _none_ | Example: `http://localhost:8080/api/v1/auth/callback`. |
| `APP_OIDC_SCOPES` | OIDC scopes requested at login. | `openid profile email` | Must include `openid`; `email` is needed for auto-provisioning. |
| `APP_OIDC_LOGIN_STATE_TTL` | Lifetime for temporary state/nonce/PKCE cookies. | `5m` | Parsed as Go duration. |
| `APP_LOGGING_LEVEL` | Zerolog level (`trace`, `debug`, `info`, `warn`, `error`). | `info` | |
| `APP_LOGGING_PRETTY` | Enable human-readable console logs. | `true` | Set to `false` for JSON logs. |
| `APP_REQUEST_LOGGER_ENABLED` | Enable request logging middleware. | `true` | |
| `APP_RECOVERY_ENABLED` | Enable panic recovery middleware. | `true` | |
| `APP_CORS_ENABLED` | Enable permissive CORS headers. | `true` | Configure or replace for stricter policies. |
| `APP_JWT_MIDDLEWARE_ENABLED` | Enable route-level JWT protection. | `true` | Set to `false` for public endpoints or local development. |

## OAuth2 / OpenID Connect

The API acts as an OIDC client. `GET /api/v1/auth/login` starts Authorization Code + PKCE login, and `GET /api/v1/auth/callback` validates the ID token, links the stable provider/subject identity, and returns a short-lived local Bearer token.

For production:

- Register the exact `APP_OIDC_REDIRECT_URL` with the provider.
- Use HTTPS for the redirect URL and a strong `APP_JWT_SECRET` of at least 32 characters.
- Keep `APP_JWT_MIDDLEWARE_ENABLED=true` so protected routes require the local Bearer token.
- Keep the default scopes unless the provider requires additional values; auto-provisioning needs a verified email claim.

## Switching Databases

1. Update `.env`:
   - For **PostgreSQL**:
     ```env
     APP_DATABASE_DRIVER=postgres
     APP_DATABASE_DSN=postgres://app:app@localhost:5432/app?sslmode=disable
     ```
   - For **MySQL**:
     ```env
     APP_DATABASE_DRIVER=mysql
     APP_DATABASE_DSN=app:app@tcp(localhost:3306)/app?parseTime=true
     ```
   - For **MongoDB**:
     ```env
     APP_DATABASE_DRIVER=mongo
     APP_MONGO_URI=mongodb://app:app@localhost:27017
     APP_MONGO_DATABASE=app
     ```

2. Start the corresponding Docker Compose service:
   ```bash
   docker compose up -d database        # PostgreSQL (default)
   docker compose --profile mysql up -d mysql
   docker compose --profile mongo up -d mongo
   ```

3. Run migrations or seed scripts matching the selected backend.

4. Restart the API process to apply changes. No code modifications are required—the database factory resolves the correct repository based on `APP_DATABASE_DRIVER`.

## Fail-Fast Validation

The configuration loader validates required settings on startup:

- Missing `APP_JWT_SECRET` returns an actionable error before Fiber starts.
- Production rejects `APP_JWT_SECRET=change-me` and secrets shorter than 32 characters.
- Enabling OIDC without issuer, client ID, client secret, redirect URL, or a positive state TTL stops boot.
- SQL drivers require `APP_DATABASE_DSN`; MongoDB requires `APP_MONGO_URI` and `APP_MONGO_DATABASE`.
- Duration values are parsed using `time.ParseDuration`; invalid formats stop boot.

Review the startup logs when adjusting configuration—errors surface with explicit messages and recommended fixes.
