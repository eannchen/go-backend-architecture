# internal/infra/db/postgres/postgrestest

## Pattern used

- Integration-only helpers start pinned disposable PostgreSQL with pgvector and apply the real Goose migrations.
- The calling test package owns one instance and explicitly closes it after all package tests finish.

## How to extend

- Reuse `Start` from a package `TestMain`; keep application-row cleanup in the test that created the rows.
- Use the returned pool for setup, assertions, and cleanup rather than creating another test-only connection.
