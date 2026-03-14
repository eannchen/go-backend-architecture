# internal/infra/db/postgres/store

## Pattern used

- Each store implements repository interfaces from `internal/repository/db/`.
- Uses generated SQL for fixed queries, shared builder for conditional queries.
- Maps results to contract types; wraps infra errors before returning.

## How to extend

- Add a store file per repository/aggregate. Implement the interface from `internal/repository/db/`.
- Map results to repository types; wrap errors with context.
