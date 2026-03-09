# internal/infra/db/postgres/store

PostgreSQL-backed implementations of repository interfaces. Contracts live in `internal/repository`.

## Pattern used

- Each store type implements one or more repository interfaces; uses the generated SQL layer for fixed queries and the shared builder for conditional queries.
- Return types and errors are mapped to contract and app error types; no DB vendor types leak.
- Tracing and errors are wrapped before leaving the package.

## How to extend

- Add a new store file per repository or aggregate; implement the interface from `internal/repository`.
- Prefer the generated SQL layer for fixed shapes; use the builder only when filters or sort are runtime-conditional.
- Map results to repository types; wrap infrastructure errors with app errors before returning.
