---
description: Testing
---

# Testing

Test behavior at the layer that owns it. Keep the subject real; replace only dependencies outside the test scope.

## Boundaries

| Scope | Keep real | Replace | Assert |
| --- | --- | --- | --- |
| Usecase unit | Usecase | Repositories | Business rules and error mapping |
| Delivery unit | Handler/middleware, binder, validator, responder | Usecases | Normalization, validation, cookies/headers, status, response mapping |
| Utility unit | Utility | Only dependencies it crosses | Public behavior and edge cases |
| Infra adapter integration | Adapter and Testcontainers backend | Unrelated external boundaries | SQL/Redis semantics, persistence, serialization, TTL, atomicity |
| HTTP feature integration | Server through delivery, usecase, repository, and infra | External providers outside the feature | Client-visible workflows and responses |
| Composition/lifecycle | Wiring and lifecycle orchestration | Expensive infrastructure outside the decision | Configuration, ordering, cleanup, error aggregation |

Test technical details in the layer that implements them. Feature tests check HTTP workflows and responses; PostgreSQL adapter tests check SQL behavior; Redis adapter tests check keys, serialization, TTL, and atomicity.

## Structure and Style

- Keep unit tests beside their source. Prefer one matching `<source>_test.go` file per source file.
- Name integration files `<subject>_integration_test.go` and use the `integration` build tag.
- Under `internal/delivery/http/integration`, put shared HTTP mechanics in `server_fixture_integration_test.go`, feature wiring/cleanup in `<feature>_fixture_integration_test.go`, and client workflows in `<feature>_flow_integration_test.go`.
- Name tests with the subject and expected result, such as `TestGetHealthRejectsInvalidQuery`. The name should explain the behavior without requiring the reader to inspect the test body.
- Use table-driven tests when cases share one arrange/act/assert flow; keep stateful or multi-step workflows explicit.
- For every bug fix, add a test in the package where the incorrect behavior originated: SQL bug -> PostgreSQL adapter test; wrong HTTP response -> handler test.
- Do not chase coverage on generated code or trivial pass-through code.

## Test Doubles

- When first needed, add one canonical configurable double for each cross-package interface under its owner's `xxxtest` child package. Do not duplicate or create doubles speculatively.
- Name it after the interface (`UserRepository`, not `MockUserRepository`). For a method such as `GetByID`, use `GetByIDFunc` to configure behavior and `GetByIDCalls` to record calls. Store arguments only when a test needs them; use typed call records when every call matters.
- Unconfigured method calls must panic so missing test setup fails immediately.
- Configure different outcomes on the canonical double. Add another type only for a distinct reusable model such as `InMemoryUserRepository`. Define a double inside one test file only for an unexported interface in that package or a special edge case that will not be reused, such as a response writer that fails on `Write`.

## Integration Lifecycle

- Use Testcontainers, not database or Redis endpoints from the environment.
- Start one container of each required type per package, normally in `TestMain`, and terminate it explicitly after the suite.
- Each test must remove its own rows and keys. Container termination cleans package resources; per-test cleanup prevents shared-container conflicts.
- Fail, never silently skip, when Docker or a required container is unavailable.

## Commands

- Focused: `go test ./path/to/package`; add `-race` for concurrency-sensitive changes.
- Default: `make test`; integration: `make test-integration`; both: `make test-all`.

