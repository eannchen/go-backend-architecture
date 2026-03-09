# internal/infra/db/postgres

PostgreSQL implementation of repository contracts. Contracts live in `internal/repository`; this package owns connection, transactions, and store implementations.

## Pattern used

- Connection lifecycle and pool in one place; transaction boundary exposed via repository contract (TxManager).
- Store subpackage implements repository interfaces using static SQL (generated) and a shared builder for dynamic SQL.
- No PostgreSQL or driver types are exposed outside this package.

## How to extend

- Add new repository implementations under the store subpackage; add static queries to the SQL layer and regenerate, or use the builder for conditional query shape.
- Schema and migrations live in the SQL layer; keep all PostgreSQL-specific logic here.
- Expose only contract types and behavior to app and usecase.
