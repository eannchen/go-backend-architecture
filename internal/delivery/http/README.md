# internal/delivery/http

## Pattern used

- Adapter pattern from HTTP transport to usecase calls.
- `server.go` owns Echo setup, global middleware, and route registration.
- Handlers under `handler/<feature>/`, files named `<feature>_<role>.go` (e.g. `auth_handler.go`, `auth_dto.go`).
- OpenAPI-generated models in `openapi/gen/` from `docs/openapi.yaml`.
- Request binding normalization in `binding/`, injected as the server's Binder.
- Request-scoped Echo context values (session, response metadata for observability) live in `httpcontext/` so handlers and middleware share one place for Set/Get helpers.

## How to extend

- Add `handler/<feature>/` with `<feature>_handler.go`, `<feature>_dto.go`, etc.
- Register routes via `RouteRegistrar`.
- Update `docs/openapi.yaml` and run `make openapi-generate` for contract changes.
- Keep handlers thin: bind/validate -> call usecase -> map response.
