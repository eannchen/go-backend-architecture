# internal/infra/db/postgres

## Pattern used

- Owns connection pool, transaction boundary (`TxManager` contract), and store implementations.
- Stores use generated static SQL and shared builder for dynamic SQL.
- No PostgreSQL/driver types exposed outside this package.

## How to extend

- Add store implementations under `store/`; static queries in the SQL layer, or builder for conditional shapes.
- Schema and migrations live in the SQL layer.
