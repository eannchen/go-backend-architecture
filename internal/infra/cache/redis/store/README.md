# Redis cache stores

This package implements optional cache contracts with Redis.

## Responsibilities and boundaries

- Each store owns its key format, serialization, expiry, and Redis command coordination.
- Redis errors are returned to the caller; the composed adapter decides whether to bypass the cache or fail the operation.
- Integration tests share one disposable Redis instance per package and clean every created key.

## Extending

- Define or update the cache contract before adding a store method.
- Use pipelines for independent batches and Lua when multiple Redis operations must be atomic.
- Test serialization, expiry, key cleanup, and Redis-specific failure behavior here.
