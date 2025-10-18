# Dependency Guide

The starter keeps dependencies minimal, focusing on packages that improve developer experience without locking teams into heavy frameworks.

| Package | Version (at generation) | Purpose | Notes |
|---------|------------------------|---------|-------|
| `github.com/gofiber/fiber/v2` | v2.52.4 | High-performance HTTP framework with familiar Express-style routing. | Wraps Fasthttp for speed and integrates easily with middleware. |
| `github.com/gofiber/jwt/v3` | v3.0.1 | JWT middleware for Fiber. | Handles token verification for protected routes. |
| `github.com/rs/zerolog` | v1.33.0 | Structured, zero-allocation logging. | Supports console pretty mode for local development. |
| `github.com/spf13/viper` | v1.18.2 | Configuration management. | Binds environment variables, defaults, and config files. |
| `github.com/joho/godotenv` | v1.5.1 | `.env` file loader for local development. | Optional in production; safe to omit when using managed secrets. |
| `github.com/jmoiron/sqlx` | v1.4.0 | SQL utilities on top of `database/sql`. | Simplifies named queries and scanning into structs. |
| `go.mongodb.org/mongo-driver` | v1.16.0 | Official MongoDB driver. | Supports context-aware operations and connection pooling. |
| `github.com/go-playground/validator/v10` | v10.20.0 | Struct validation library. | Used for DTO validation in HTTP handlers. |
| `github.com/stretchr/testify` | v1.9.0 | Testing helpers and assertions. | Encourages fluent unit tests. |
| `github.com/DATA-DOG/go-sqlmock` | v1.5.2 | SQL mocking for repository tests. | Enables deterministic testing without live databases. |

## Managing Versions

- Versions listed above reflect the state when this starter was generated. Use `go get -u` or `renovate` to automate upgrades.
- The project targets Go 1.22; ensure CI environments use the same toolchain to avoid module resolution mismatches.
- Prefer semantic version bumps. Breaking changes should be reviewed by maintainers before adoption.

## Adding New Dependencies

1. Confirm the package aligns with clean architecture boundaries (domain vs infrastructure).
2. Run `go get <module>@<version>` and `go mod tidy` to update `go.mod` and `go.sum`.
3. Update this document with rationale and version information.
4. If the dependency provides developer tooling (linters, generators), document usage in `docs/architecture/overview.md` or `README.md`.

Keep the dependency surface lean to maintain quick builds and reduce security maintenance overhead.
