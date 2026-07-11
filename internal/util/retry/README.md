# internal/util/retry

## Pattern used

- Context-aware bounded retry for transient operations.
- Optional retry filtering and callbacks keep policy visible at the call site.

## How to extend

- Keep retries finite and respect the caller's context.
- Put operation-specific retry decisions in `ShouldRetry`; do not embed provider policy here.
