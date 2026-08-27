# internal/infra/redisconn/redistest

## Pattern used

- Integration-only helpers start a pinned disposable Redis container and return a real client.
- Each calling package owns a separate container and explicitly closes it after `m.Run`.

## How to extend

- Reuse `RunPackage` from a package `TestMain`; do not share one container across Go packages.
- Keep test-key cleanup in the test that created the key so isolation remains visible.
