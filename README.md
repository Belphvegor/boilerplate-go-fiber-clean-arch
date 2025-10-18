# Clean Architecture Fiber Starter

A production-ready Go starter leveraging [Fiber](https://gofiber.io) with clean architecture boundaries, multi-database configuration, and structured documentation. It ships with a fully wired User domain as an example, automation scripts, and guidance to help teams extend the project confidently.

## Highlights

- Clean architecture layering with explicit boundaries between delivery, use cases, entities, and infrastructure.
- Environment-driven configuration with fail-fast validation for PostgreSQL, MySQL, or MongoDB backends.
- Structured logging via Zerolog and configurable Fiber middleware (request logging, CORS, recovery, JWT).
- Example `User` bounded context demonstrating repository interfaces, SQL & Mongo adapters, and HTTP handlers.
- Docker Compose environment for local development and integration tests.
- Comprehensive documentation under `docs/` including architecture overview, folder reference, configuration matrix, and quickstart workflow.

## Getting Started

1. **Create your environment file**
   ```bash
   cp .env.example .env
   ```
2. **Install dependencies**
   ```bash
   go mod tidy
   ```
3. **Start supporting services**
   ```bash
   docker compose up -d database
   ```
4. **Run the API**
   ```bash
   go run ./cmd/api
   ```

Detailed setup steps and troubleshooting tips are available in [`docs/quickstart.md`](docs/quickstart.md).

## Project Structure

```
cmd/api/                     Application entrypoint
config/                      Configuration loader and validation
internal/bootstrap/          Dependency container and module registration
internal/user/               Sample User bounded context (entity, use case, delivery, repositories)
pkg/                         Shared packages (database, logger, middleware, response)
db/                          Database migrations and seed data
docs/                        Architecture, configuration, and API documentation
scripts/                     Automation scripts (Makefile targets)
test/                        Integration and end-to-end tests
```

Refer to [`docs/architecture/folder-reference.md`](docs/architecture/folder-reference.md) for a detailed directory catalog with extension notes.

## Development Workflow

Common automation commands live in `scripts/Makefile`:

```bash
make -C scripts build        # Compile the application
make -C scripts fmt          # Format Go code
go test ./...                # Run unit tests
go run ./cmd/api             # Start the Fiber server
```

Additional commands cover linting (`make -C scripts lint`) and database migrations (`make -C scripts migrate-up`). Adjust or extend the Makefile to match your team's tooling preferences.

## Documentation Map

- [`docs/architecture/overview.md`](docs/architecture/overview.md) — clean architecture principles and request lifecycle
- [`docs/architecture/domain-checklist.md`](docs/architecture/domain-checklist.md) — step-by-step guide to add new bounded contexts
- [`docs/architecture/dependencies.md`](docs/architecture/dependencies.md) — rationale behind selected libraries and version guidance
- [`docs/configuration.md`](docs/configuration.md) — configuration matrix and environment-driven behaviours (added in User Story 3)
- [`docs/observability.md`](docs/observability.md) — logging levels, middleware toggles, and monitoring hooks (added in User Story 3)
- [`docs/swagger/user.yaml`](docs/swagger/user.yaml) — OpenAPI specification for the sample User module

## Contributing

1. Fork and clone the repository.
2. Create a feature branch following the naming convention from `/specs` (e.g., `001-specify-scripts-bash`).
3. Use the task list in [`specs/001-specify-scripts-bash/tasks.md`](specs/001-specify-scripts-bash/tasks.md) to plan implementation.
4. Ensure `go test ./...` passes and run integration tests via Docker Compose before opening a pull request.
5. Document any new modules or configuration updates in `docs/`.

---

This starter is maintained as part of the Clean Architecture learning sandbox. Contributions, issues, and suggestions are welcome.
