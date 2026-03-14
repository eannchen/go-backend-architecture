# internal/infra/db

## Pattern used

- One subpackage per database vendor. Implements contracts from `internal/repository/db/`.
- Static SQL in a generated layer; dynamic queries via shared builder. All SQL stays in infra.
- No DB types leak to usecase.

## How to extend

- Add subpackage per database/vendor implementing repository contracts.
- Prefer generated SQL for fixed shapes; builder only for runtime-conditional filters/sort.
