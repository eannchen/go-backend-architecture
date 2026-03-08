# internal/infra/db/postgres/sqlc

sqlc source and generated code for PostgreSQL.

## Pattern used

- SQL source query files live in this folder (`*.sql`); generated Go lives in `gen/`.
- Schema is the single source of truth in `schema.sql` for sqlc code generation.

## How to extend

- Add or edit `.sql` query files, then run sqlc generate.
- Update `schema.sql` for DDL changes; run DB migrations separately (see parent package `migrations/`).
