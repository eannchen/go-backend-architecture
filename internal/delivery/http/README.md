# internal/delivery/http

Echo HTTP server and handlers.

## Pattern used

- Adapter pattern from HTTP transport to usecase calls.
- `server.go` owns Echo setup, global middleware, and route registration flow.
- Feature handlers live under `http/<feature>/` and implement `RouteRegistrar`.
- OpenAPI-driven transport models can be generated under `http/openapi/gen/` from `docs/openapi.yaml`.
- Response mapping and validation stay in delivery layer.

## How to extend

- Add new feature package under `http/<feature>/`.
- Register routes via feature `RouteRegistrar` implementations.
- Update `docs/openapi.yaml` and regenerate `http/openapi/gen/` when HTTP contracts change.
- Keep handler thin: bind/validate -> call usecase -> map response/error.
- Do not place business rules in this package.
