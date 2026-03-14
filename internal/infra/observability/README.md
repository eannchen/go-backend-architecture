# internal/infra/observability

## Pattern used

- Contract (tracing, metrics, log emission) in `internal/observability`; this package provides the implementation.
- All vendor/SDK details stay here. Lifecycle (startup/shutdown) is part of implementation.

## How to extend

- Extend contract in `internal/observability` when new capability is needed.
- Add/change implementation here; ensure lifecycle hooks are wired from app.
