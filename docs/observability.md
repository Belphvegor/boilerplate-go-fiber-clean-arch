# Observability & Middleware

The starter publishes structured logs and enables core middleware by default. Use this guide to tune runtime behaviour and hook external observability systems.

## Logging

- **Library**: [Zerolog](https://github.com/rs/zerolog)
- **Configuration**: Controlled by `APP_LOGGING_LEVEL` and `APP_LOGGING_PRETTY`.
  - `trace`/`debug` reveal verbose developer diagnostics.
  - `info` is the production default.
  - `warn`/`error` reduce noise when piping logs to aggregation systems.
  - Set `APP_LOGGING_PRETTY=false` for JSON output compatible with ELK/Datadog.
- **Contextual Fields**: Every log entry includes `timestamp` and `service`. Request logging middleware appends `ip`, `method`, `path`, `status`, and latency.

## Middleware Toggles

| Flag | Default | Description |
|------|---------|-------------|
| `APP_REQUEST_LOGGER_ENABLED` | `true` | Uses Fiber logger middleware with standardized formatting. Disable when replacing with custom instrumentation. |
| `APP_RECOVERY_ENABLED` | `true` | Captures panics and returns HTTP 500 with a safe payload. |
| `APP_CORS_ENABLED` | `true` | Applies permissive CORS headers (`*`). Replace with project-specific policy if needed. |
| `APP_JWT_MIDDLEWARE_ENABLED` | `true` | Enforces JWT authentication on routes registered after middleware. Set to `false` for fully public APIs or local prototyping. |

## JWT Middleware

The JWT middleware uses `APP_JWT_SECRET`, `APP_JWT_ISSUER`, and `APP_JWT_ACCESS_TTL` to validate tokens. Supply an `Authorization: Bearer <token>` header when enabled. Handlers can access claims via `ctx.Locals("user")`.

## Metrics & Tracing Hooks

- The middleware register provides a single location (`pkg/middleware/http.go`) to insert metrics or tracing providers (e.g., Prometheus, OpenTelemetry).
- Toggle `APP_DATABASE_METRICS_ENABLED` to gate future database observability integrations.
- Add custom middleware before route registration to enrich request context (e.g., correlation IDs).

## Health & Readiness

- `/healthz` returns `200 OK` when the server is operational.
- Extend this endpoint with dependency checks (databases, external APIs) by introducing additional health packages in `internal/shared`.

## Production Recommendations

1. **Centralized Logging**: Configure JSON output and ship logs to your aggregation tool (ELK, Loki, Datadog).
2. **Structured Trace Context**: Add request ID middleware and propagate IDs through use cases and repositories.
3. **Metrics**: Integrate Fiber's Prometheus middleware or OpenTelemetry exporters and expose metrics via `/metrics`.
4. **Security**: Rotate JWT secrets regularly and store them in a secret manager; never commit `.env` to source control.
5. **Audit**: Wrap domain-specific events with additional structured logging for compliance workflows.
