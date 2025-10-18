# Domain Extension Checklist

Use this checklist when introducing a new bounded context. Each step keeps the clean architecture layering intact and ensures documentation stays up to date.

1. **Clone the structure**
   - Duplicate `internal/user` as `internal/<domain>` and remove implementation-specific files.
   - Replace names within entity, repository, use case, delivery, and persistence folders.

2. **Define the entity**
   - Model core fields and invariants in `internal/<domain>/entity`.
   - Add validation rules and factory helpers as needed.

3. **Specify repository contracts**
   - Declare interfaces and error variables in `internal/<domain>/repository`.
   - Keep interfaces minimal and use domain language for method names.

4. **Implement use cases**
   - Create services in `internal/<domain>/usecase` orchestrating domain logic.
   - Inject repository interfaces, validator, and logger dependencies from the container.

5. **Wire persistence adapters**
   - Implement SQL repositories under `internal/<domain>/infrastructure/persistence` using `sqlx`.
   - Provide MongoDB adapters when supported.
   - Add migrations to `db/migrations` and seed data to `db/seed` if required.

6. **Expose delivery endpoints**
   - Define request/response DTOs in `internal/<domain>/delivery/http/dto.go`.
   - Implement handlers and register routes in `internal/<domain>/delivery/http/routes.go`.
   - Update OpenAPI documentation in `docs/swagger/<domain>.yaml`.

7. **Register module**
   - Call the domain route registrar within `internal/bootstrap` (see `internal/user/delivery/http/routes.go`).
   - Add dependency wiring in the bootstrap container if new services are required.

8. **Test coverage**
   - Write unit tests for use cases and repositories (using `sqlmock` or test containers).
   - Add integration tests under `test/integration` covering critical flows.

9. **Documentation updates**
   - Extend configuration docs if new environment variables are introduced.
   - Update README and architecture overview with module highlights.

10. **Verify automation**
   - Run `make -C scripts fmt`, `make -C scripts lint`, and `make -C scripts test` before submitting changes.

Mark each step complete to ensure the new module aligns with project standards.
