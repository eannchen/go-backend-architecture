# PostgreSQL stores

This package implements database repository contracts with PostgreSQL.

## Responsibilities and boundaries

- Uses sqlc output for fixed queries and the shared builder for runtime-conditional query shapes.
- Maps database rows and driver errors to domain types, repository projections, and repository sentinel errors.
- Owns transaction boundaries required to complete one repository operation atomically.
- Integration tests share a disposable PostgreSQL instance per package and clean rows created by each test.

## Extending

- Add one store file per cohesive repository capability.
- Add the repository contract first, then implement only the operations it requires.
- Test SQL behavior, constraints, transactions, and error mapping in this package.
