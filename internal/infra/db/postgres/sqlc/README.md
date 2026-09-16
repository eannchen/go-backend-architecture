# internal/infra/db/postgres/sqlc

## Pattern used

- Query and schema files are source of truth; generated Go code stays inside infra.
- No string concatenation for SQL.

## How to extend

- Add/edit query files, then run `make sqlc-generate` with the installed profile configuration.
- Keep authentication tables in `schema/auth.sql`; the gRPC health queries need no application schema.
