# Redis key-value stores

This package uses Redis as the primary store for sessions, one-time passwords (OTPs), OAuth authorization state, and distributed rate-limit state.

## Responsibilities and boundaries

- Each store owns key format, serialization, expiry, and command coordination for one capability.
- Lua scripts provide atomic multi-step operations such as distributed rate-limit checks.
- Integration tests share one disposable Redis instance per package and clean every created key.

## Extending

- Define or update the key-value contract before adding a store operation.
- Use Lua for atomic flows and pipelines for independent batched commands.
- Test expiry, key cleanup, serialization, and concurrency-sensitive behavior here.
