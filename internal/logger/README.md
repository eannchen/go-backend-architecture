# internal/logger

Logging contract used by usecase and delivery. Implementations live in infra.

## Pattern used

- Consumer-owned interface: usecase and delivery depend on `Logger`, not on a concrete implementation.
- No infra import in app layers: only this package is imported for logging.

## How to extend

- Add new methods to `Logger` only when needed by app layers.
- Implement under `internal/infra/logger/zap` or another infra logger; wire in app.
