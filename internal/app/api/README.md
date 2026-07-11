# internal/app/api

## Pattern used

- Composes the HTTP API process from the shared `runtime` dependencies.
- Builds in stages: infra -> repositories -> usecases -> handlers -> server.
- Returns startup errors early and cleans up already-initialized resources.

## How to extend

- Add HTTP-specific constructor wiring in the matching `api_*_wiring.go` file.
- Keep process-neutral setup in `internal/app/runtime` and business logic outside this package.
