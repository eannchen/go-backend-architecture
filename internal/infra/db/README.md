# internal/infra/db

Database adapters and helpers.

## Pattern used

- Adapter pattern for persistence concerns.
- `builder` holds dynamic SQL helpers (`Squirrel`) used by repo implementations.
- `postgres` contains PostgreSQL-specific wiring and repository implementations.

## How to extend

- Add a new subpackage per DB/vendor concern (example: `mysql`, `clickhouse`).
- Keep repository contracts in `internal/repository`; implement them here.
- Keep vendor-specific imports inside `internal/infra/db/...` packages.
