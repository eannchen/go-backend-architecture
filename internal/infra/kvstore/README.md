# internal/infra/kvstore

Key-value store adapters.

## Pattern used

- Adapter implementations for focused `internal/repository` KV contracts.
- Provider-specific code under subpackages (`redis/`, future others).
- For Redis: keep client construction in `redis/connection.go`, and concrete stores in `redis/store/`.

## How to extend

- Add provider package (`dynamodb/`, `badger/`, etc.) implementing the same contracts.
- Keep provider command/API usage isolated to this layer.
- Keep business semantics explicit in contract names (for example `IdempotencyStore`), not generic "bag of key-values".
- Keep each concrete capability in its own store file under `*/store/` for consistent structure.
