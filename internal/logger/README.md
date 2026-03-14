# internal/logger

## Pattern used

- Consumer-owned logging interface. Usecase and delivery depend on `Logger`, not concrete implementations.

## How to extend

- Add methods to `Logger` only when needed by app layers.
- Implement under `internal/infra/logger/`; wire in app.
