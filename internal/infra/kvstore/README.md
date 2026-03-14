# internal/infra/kvstore

## Pattern used

- One subpackage per KV backend. Implements contracts from `internal/repository/kvstore/`.
- Backend usage and key semantics stay here; usecases depend only on interfaces.

## How to extend

- Add subpackage per backend implementing the same repository contracts.
- Keep each capability in its own store file.
