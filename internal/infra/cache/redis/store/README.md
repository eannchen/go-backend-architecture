# internal/infra/cache/redis/store

## Pattern used

- Each store owns its Redis key layout, serialization, TTL, and command coordination.
- One store method per business operation. Redis primitives (Lua, pipelines) stay inside store methods.

## How to extend

- Add/update a contract in `internal/repository/cache/` first, then implement here.
- Use Lua scripts for atomic multi-key flows; pipelines for batching independent commands.
