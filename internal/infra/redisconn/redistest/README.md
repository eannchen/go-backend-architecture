# Redis integration fixtures

This package starts and manages disposable Redis instances for integration-test packages.

## Responsibilities and boundaries

- Uses a pinned container image and returns a real Redis client.
- Each calling package owns and closes its own container.
- Failed suites can copy container logs before shutdown so CI retains diagnostics.

## Extending

- Use `RunPackage` for Redis-only suites or `Start` when composing multiple dependencies in `TestMain`.
- Keep key cleanup in the test that created each key.
- When using `Start`, call `WriteLogs` after failure and before `Close`.
