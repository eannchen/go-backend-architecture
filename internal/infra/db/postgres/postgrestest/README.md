# internal/infra/db/postgres/postgrestest

## Pattern used

- Integration-only helpers start pinned disposable PostgreSQL with pgvector and apply the real Goose migrations.
- The calling test package owns one instance and explicitly closes it after all package tests finish.
- Failed suites copy container logs before cleanup so CI keeps the diagnostics.

## How to extend

- Reuse `Start` from a package `TestMain`; keep application-row cleanup in the test that created the rows.
- Use the returned pool for setup, assertions, and cleanup rather than creating another test-only connection.
- Call `WriteLogs` only after a failed suite and before `Close`.
