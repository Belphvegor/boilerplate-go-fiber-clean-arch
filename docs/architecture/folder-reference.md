# Folder Reference

A quick lookup for contributors to understand where to place new code or documentation. Paths are relative to the repository root.

| Path | Description | Common Additions |
|------|-------------|------------------|
| `cmd/api/` | Entrypoint for the HTTP server. Contains `main.go` and startup wiring only. | New binaries (e.g., CLI) live under `cmd/<name>/`. |
| `config/` | Configuration loaders, defaults, and validation. | Additional configuration helpers or environment adapters. |
| `internal/bootstrap/` | Dependency container, route registration, and lifecycle hooks. | Register new modules, background workers, or graceful shutdown logic. |
| `internal/<domain>/entity/` | Domain entities with invariants. | Struct definitions, validation helpers, factory functions. |
| `internal/<domain>/usecase/` | Application services orchestrating domain behaviour. | New use case structs, command/query handlers, transactional logic. |
| `internal/<domain>/repository/` | Interfaces describing persistence operations. | Additional repository methods or error definitions. |
| `internal/<domain>/infrastructure/persistence/` | Concrete persistence adapters (SQL, MongoDB, external APIs). | Extra adapters, migrations glue, caching layers. |
| `internal/<domain>/delivery/http/` | HTTP DTOs, handlers, and route wiring. | New endpoints, request validation, versioned routes. |
| `internal/shared/` | Shared utilities (validator, errors, middleware helpers). | Add cross-cutting helpers reusable by multiple domains. |
| `pkg/logger/` | Structured logging factory and composable logging helpers. | Additional log formatting or sinks. |
| `pkg/database/` | Database factory for SQL and MongoDB connections. | Support for more drivers or connection instrumentation. |
| `pkg/middleware/` | Fiber middleware registration and toggles. | Custom middleware, metrics instrumentation. |
| `pkg/response/` | Standardized API response helpers. | Envelope variants, pagination helpers. |
| `db/migrations/` | Schema migrations. | SQL migration files, seed scripts. |
| `db/seed/` | Seed data or fixtures. | Sample data for integration tests. |
| `docs/` | Product and developer documentation. | Architecture notes, quickstart guides, Swagger specs. |
| `scripts/Makefile` | Automation commands for build/test/lint/migrations. | Additional workflow targets. |
| `test/integration/` | Integration tests hitting external dependencies. | New integration suites aligned with user stories. |
| `test/e2e/` | End-to-end acceptance tests. | Black-box tests exercising API flow. |

Keep new code within the appropriate sub-tree to preserve clean architecture boundaries. If a directory is missing for your new domain or capability, mirror the `internal/user` structure to maintain consistency.
