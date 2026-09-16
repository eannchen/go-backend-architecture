# internal/infra/db/postgres/store

## Pattern used

- Each store implements repository interfaces from `internal/repository/db/`.
- Uses generated SQL for fixed queries, shared builder for conditional queries.
- Maps database results to domain entities or repository projections; wraps infra errors before returning.
- Integration tests share one disposable PostgreSQL container per package, run real migrations once, and clean up rows after each test.

## How to extend

- Add a store file per repository/aggregate. Implement the interface from `internal/repository/db/`.
- Map results to domain entities or repository projections; wrap errors with context.
- Add integration cases to the package harness and register `t.Cleanup` for every row the test creates.
