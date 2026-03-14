# internal/infra/db/postgres/sqlc

## Pattern used

- Query files and schema are source of truth; generated Go code in a subpackage, only imported by infra.
- No string concatenation for SQL.

## How to extend

- Add/edit query files, then run the sqlc generate step.
- For schema changes, update schema file and run migrations; keep DDL consistent.
