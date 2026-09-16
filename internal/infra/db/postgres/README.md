# internal/infra/db/postgres

## Pattern used

- Owns connection pool, transaction boundary (`TxManager` contract), and store implementations.
- Stores use generated static SQL and shared builder for dynamic SQL.
- No PostgreSQL/driver types exposed outside this package.
- `pgtype.go` converts PostgreSQL driver values at the infra boundary.
- **DDL:** `sqlc/schema/auth.sql` mirrors the public HTTP authentication tables; migrations apply them in environments.

## How to extend

- Add store implementations under `store/`; static queries in the SQL layer, or builder for conditional shapes.
- Use the `pgtype` helpers when mapping nullable timestamps or date-only values to domain-friendly Go types.
- Add migrations under `migrations/`; mirror structural changes in the matching `sqlc/schema/` file, then run `make sqlc-generate`.
