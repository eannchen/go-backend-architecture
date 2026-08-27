# internal/infra/kvstore/redis/store

## Pattern used

- Each store owns its Redis key layout, serialization, TTL, and command coordination.
- One store method per business operation. Redis primitives stay inside store methods.
- Rate-limit stores use Lua so a distributed check-and-record operation is atomic.
- Integration tests share one disposable Redis container per package and fail if a test leaks keys.

## How to extend

- Add/update a contract in `internal/repository/kvstore/` first, then implement here.
- Use Lua scripts for atomic multi-key flows; pipelines for batching independent commands.
- Register `t.Cleanup` for every integration-test key and assert TTL or atomic behavior where owned by Redis.
