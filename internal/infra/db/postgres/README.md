# internal/infra/db/postgres

PostgreSQL-specific implementations.

- Creates pgx pool and DB lifecycle management.
- `repos`: repository implementations using sqlc (static) + Squirrel (dynamic).
- `sqlc`: generated PostgreSQL sqlc code.
- `migrations`: PostgreSQL migration files (goose).
