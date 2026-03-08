# internal/infra/kvstore

Key-value store adapters.

## Pattern used

- Adapter implementations for `internal/repository` KV contracts.
- Provider-specific code under subpackages (`redis/`, future others).

## How to extend

- Add provider package (`dynamodb/`, `badger/`, etc.) implementing the same contracts.
- Keep provider command/API usage isolated to this layer.
