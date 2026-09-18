# PostgreSQL integration fixtures

This package starts and manages disposable PostgreSQL instances for integration-test packages.

## Responsibilities and boundaries

- Uses a pinned container image and applies the project's SQL migrations with Goose.
- The calling package owns one instance for its test process and closes it after `m.Run`.
- Failed suites can copy container logs before shutdown so CI retains diagnostics.

## Extending

- Start the fixture from package `TestMain` and reuse its pool for setup, assertions, and cleanup.
- Keep application-row cleanup in the test that created the rows.
- Call `WriteLogs` after a failed suite and before `Close`.
