# internal/infra/cache/redis/store

Redis-backed cache stores.

## Pattern used

- Each store owns its Redis key layout, serialization, TTL policy, and command coordination.
- Keep Redis primitives such as Lua scripts and pipelines inside store methods; do not leak them into usecases or repository contracts.
- Prefer one store method per business operation. Do not introduce a generic Redis transaction manager (like `repository.TxManager`) unless several Redis-backed repositories truly need one shared execution boundary.

## How to extend

- Add or update a business-oriented repository contract first in `internal/repository`.
- Implement the contract in this package and keep Redis-specific logic local to the store.
- Use Lua scripts for atomic multi-key read-check-write flows.
- Use pipelines for batching independent commands where reducing round trips matters more than cross-command rollback.
