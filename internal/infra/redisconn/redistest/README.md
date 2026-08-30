# internal/infra/redisconn/redistest

## Pattern used

- Integration-only helpers start a pinned disposable Redis container and return a real client.
- Each calling package owns a separate container and explicitly closes it after `m.Run`.
- Failed suites copy container logs before cleanup so CI keeps the diagnostics.

## How to extend

- Reuse `RunPackage` for Redis-only suites, or `Start` when composing multiple dependencies in one package `TestMain`.
- Do not share one container across Go packages.
- Keep test-key cleanup in the test that created the key so isolation remains visible.
- `RunPackage` handles failure logs; callers using `Start` must call `WriteLogs` before `Close` when their suite fails.
