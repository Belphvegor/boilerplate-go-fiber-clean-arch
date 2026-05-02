# Repository Guidelines

## Project Structure & Module Organization
The Go application lives under `cmd/api` (entrypoint) and `internal/` (bootstrap wiring and bounded contexts such as `internal/user`). Shared helpers stay in `pkg/` (`logger`, `middleware`, `response`). Configuration loaders are stored in `config/`, while database migrations and seed data sit in `db/`. Documentation and architectural references are under `docs/`. Integration and end-to-end test suites belong in `test/`, and build tooling resides in `scripts/`.

## Build, Test, and Development Commands
- `make -C scripts build` - compile the service across modules.
- `make -C scripts fmt` - format Go sources with `gofmt`.
- `make -C scripts lint` - run `golangci-lint` (falls back to `go vet`).
- `go run ./cmd/api` - launch the Fiber server locally.
- `go test ./...` - execute unit and integration tests.
- `docker compose up -d database` - start backing services for local runs.

## Coding Style & Naming Conventions
Target Go 1.22 with idiomatic style. Keep files formatted via `make -C scripts fmt` before every commit. Prefer package-level names that reflect architecture layers (e.g., `userRepository`, `CreateUserUseCase`). Interfaces normally live near their consumers inside `internal/<domain>`, and shared types should only be exported from `pkg/`. Structured logging flows through Zerolog helpers; avoid ad-hoc `fmt.Println`.

## Testing Guidelines
Author tests first to satisfy the "Tests Before Trust" principle. Store unit tests alongside their packages and use `sqlmock` or containerized databases for persistence coverage. Name integration tests after the user flow they verify (e.g., `TestUserCreateHandler_Success`). Run `go test ./...` before pushing and attach benchmark or load scripts when you touch hot paths.

## Commit & Pull Request Guidelines
Write imperative, focused commit messages that describe the outcome (e.g., `docs: update domain checklist for audit logging`). Each pull request must confirm formatting, lint, and test passes, outline architecture impacts, link relevant specs in `specs/<feature-id>/`, and include documentation or schema changes in the same branch.

## Security & Configuration Tips
Clone `.env.example` to `.env` for local secrets and never commit actual credentials. Document any new environment key in `docs/configuration.md` and wire defaults through `config/`. Use `pkg/middleware` for standardized observability and ensure Zerolog fields include correlation IDs when handling requests.
