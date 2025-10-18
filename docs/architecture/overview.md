# Architecture Overview

The Clean Architecture Fiber Starter provides a layered Go backend that keeps framework concerns at the edge while protecting core business logic. Each request flows through delivery adapters, into orchestrated use cases, and interacts with infrastructure components through explicit contracts.

## Guiding Principles

- **Dependency inversion**: Inner layers (`entity`, `usecase`) never depend on outer layers (`delivery`, `infrastructure`). Interfaces define the contracts, and infrastructure packages implement them.
- **Testability first**: Business rules live in use cases that can be unit tested without networking or database access. Infrastructure is swapped out by mocks in tests.
- **Configuration driven**: Environment variables and the configuration loader shape runtime behaviour (database driver, logging level, middleware toggles) without code changes.
- **Observability readiness**: Structured logging, request tracing hooks, and health endpoints ship by default so teams can add metrics and tracing with minimal effort.

## Layer Summary

| Layer | Directory | Responsibilities |
|-------|-----------|------------------|
| Delivery | `internal/<domain>/delivery/http` | HTTP handlers, request/response DTOs, routing |
| Use Case | `internal/<domain>/usecase` | Application services orchestrating entities and repositories |
| Domain Entity | `internal/<domain>/entity` | Pure data structures and invariants |
| Repository Interface | `internal/<domain>/repository` | Contracts describing persistence behaviour |
| Infrastructure | `internal/<domain>/infrastructure/persistence` | SQL / MongoDB implementations behind repository interfaces |
| Shared Packages | `pkg/` | Cross-cutting concerns (logger, database, middleware, responses) |
| Bootstrap | `internal/bootstrap` | Dependency container wiring configuration, logging, and database |

## Request Lifecycle

1. Fiber receives a request and executes shared middleware (recovery, request logging, CORS, JWT validation) configured in `pkg/middleware`.
2. A route defined within a domain module delegates to an HTTP handler.
3. The handler validates input DTOs, invokes the corresponding use case, and returns standardized responses using `pkg/response` helpers.
4. Use cases depend on repository interfaces and emit structured logs through the shared logger instance.
5. Repository implementations talk to SQL (`sqlx`) or MongoDB clients provided by the database factory and return domain entities to the use case.

## Module Independence

Each domain folder is self-contained. Adding a new domain means duplicating the `internal/user` layout, updating bootstrap wiring, and documenting new contracts. The starter keeps transverse concerns (`validator`, `middleware`, `logger`) in dedicated packages to avoid accidental coupling.

## Deployment Model

The service runs as a single Fiber binary and connects to databases defined in `docker-compose.yml`. Teams can containerize the binary with the provided `.dockerignore` defaults. Health endpoints support orchestration readiness probes, while environment validation catches misconfiguration before the server starts accepting traffic.

## Migration Strategy

Database schema migrations live in `db/migrations` and are executed through Make targets or the `migrate` container. The starter does not mandate a specific tooling binary, allowing teams to integrate Flyway, Goose, or Atlas as needed.
