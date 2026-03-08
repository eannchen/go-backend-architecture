# internal/app

Composition root for dependency injection.

## Pattern used

- Manual DI via constructor wiring (no service locator).
- Build in stages: infra stores -> repositories -> usecases -> handlers -> server.
- Return startup errors early and clean up already-initialized resources.
- Keep provider-specific adapter assembly in wiring helpers (for example Redis store bundle).

## How to extend

- Add constructor wiring in `wiring.go` only.
- Keep business logic out of this package.
- Keep initialization deterministic and testable.
