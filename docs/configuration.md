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
| `APP_JWT_ACCESS_TTL` | Access token lifetime. | `15m` | Parsed as Go duration. |
| `APP_LOGGING_LEVEL` | Zerolog level (`trace`, `debug`, `info`, `warn`, `error`). | `info` | |
| `APP_LOGGING_PRETTY` | Enable human-readable console logs. | `true` | Set to `false` for JSON logs. |
| `APP_REQUEST_LOGGER_ENABLED` | Enable request logging middleware. | `true` | |
| `APP_RECOVERY_ENABLED` | Enable panic recovery middleware. | `true` | |
| `APP_CORS_ENABLED` | Enable permissive CORS headers. | `true` | Configure or replace for stricter policies. |
| `APP_JWT_MIDDLEWARE_ENABLED` | Enable JWT middleware for protected routes. | `true` | Set to `false` for public endpoints or local development. |

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
- SQL drivers require `APP_DATABASE_DSN`; MongoDB requires `APP_MONGO_URI` and `APP_MONGO_DATABASE`.
- Duration values are parsed using `time.ParseDuration`; invalid formats stop boot.

Review the startup logs when adjusting configuration—errors surface with explicit messages and recommended fixes.
