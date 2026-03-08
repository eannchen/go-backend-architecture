# internal/infra/db/postgres

PostgreSQL-specific implementations.

## Pattern used

- Infrastructure adapter package for PostgreSQL only.
- `connection.go` handles `pgx` pool lifecycle.
- `tx_manager.go` owns transaction boundaries for usecases.
- `store` implements repository contracts with `sqlc` (static SQL).

## How to extend

- Add new PostgreSQL repository implementations under `store/`.
- Add static queries under `sqlc/*.sql`, then regenerate code.
- Add schema/migration changes under `sqlc/schema.sql` and `migrations/`.
- Keep PostgreSQL-only details in this package; expose contract-friendly behavior upward.
