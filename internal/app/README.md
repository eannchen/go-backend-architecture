# internal/app

## Pattern used

- Manual DI via constructor wiring (no service locator).
- Build in stages: infra -> repositories -> usecases -> handlers -> server.
- Return startup errors early; clean up already-initialized resources.

## How to extend

- Keep shared dependencies in `wiring.go`; add constructor wiring in the matching `*_wiring.go` file (`stores`, `repositories`, `usecases`, `handlers`, or `server`).
- Keep business logic out of this package.
