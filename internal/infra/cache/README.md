# internal/infra/cache

Cache adapters.

## Pattern used

- Adapter implementations for cache capabilities.
- Provider-specific code under subpackages (`redis/`, future others).
- For Redis: keep client construction in `redis/connection.go`, and concrete stores in `redis/store/`.

## How to extend

- Keep cache interfaces small and focused (for example: read model cache, health ping).
- Add new provider packages (`memcached/`, `ristretto/`) implementing the same contracts.
- Keep serialization and key-policy logic in this layer, not in usecases.
- Keep each concrete capability in its own store file under `*/store/` for consistent structure.
