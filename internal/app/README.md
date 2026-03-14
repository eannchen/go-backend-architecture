# internal/app

## Pattern used

- Manual DI via constructor wiring (no service locator).
- Build in stages: infra -> repositories -> usecases -> handlers -> server.
- Return startup errors early; clean up already-initialized resources.

## How to extend

- Add constructor wiring in `wiring.go`.
- Keep business logic out of this package.
