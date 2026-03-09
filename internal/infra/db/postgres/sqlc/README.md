# internal/infra/db/postgres/sqlc

SQL source and generated Go for static PostgreSQL queries. Schema and queries are the source of truth; generated code lives in a subpackage.

## Pattern used

- Query files and schema live here; generated code is kept in a dedicated subpackage so that only infra imports it.
- Static query shape is defined in SQL; no string concatenation for SQL in the project.

## How to extend

- Add or edit query files, then run the project’s sqlc generate step.
- For schema changes, update the schema file and run migrations via the project’s migration path; keep DDL and migrations consistent with this schema.
