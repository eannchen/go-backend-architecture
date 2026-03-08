# internal/infra/db/postgres/store

PostgreSQL-backed store implementations for repository contracts.

- Implements interfaces from `internal/repository`.
- Current template example: account summary read model + DB health checks.
- Prefer `sqlc` static queries for predictability and reviewability.
