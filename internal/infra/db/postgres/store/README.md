# internal/infra/db/postgres/store

PostgreSQL-backed store implementations for repository contracts.

## Pattern used

- Implements interfaces from `internal/repository`.
- Uses `sqlc` for static queries and `builder` (`squirrel`) for dynamic query shape.

## How to extend

- Prefer `sqlc` for fixed query shape (`GetByID`, health status, etc.).
- Use `builder` only when filters/sort/paging are runtime-conditional.
- Keep return models mapped to repository contracts, not DB vendor types.
