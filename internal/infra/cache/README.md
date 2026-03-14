# internal/infra/cache

## Pattern used

- One subpackage per cache backend. Implements cache contracts from `internal/repository/cache/`.
- Serialization and key policy stay in this layer; usecases depend only on interfaces.

## How to extend

- Add subpackage per backend implementing the same repository contracts.
- Keep interfaces small in `internal/repository/cache/`; add store files for new capabilities.
