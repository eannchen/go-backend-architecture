# internal/delivery/http

## Pattern used

- Adapter pattern from HTTP transport to usecase calls.
- `server.go` owns Echo setup, global middleware, and route registration.
- Handlers under `handler/<feature>/`, files named `<feature>_<role>.go` (e.g. `auth_handler.go`, `auth_dto.go`).
- OpenAPI-generated request and response models in `openapi/gen/` from `contracts/http/openapi.yaml`.
- Handlers bind and validate generated request models explicitly; business validation remains in usecases and domain types.
- Request binding normalization in `binding/`, injected as the server's Binder.
- Request-scoped Echo context values (session, response metadata for observability) live in `httpcontext/` so handlers and middleware share one place for Set/Get helpers.
- One observability middleware coordinates tracing, metrics, and access logging from a shared request outcome.

## How to extend

- Add `handler/<feature>/` with `<feature>_handler.go`, `<feature>_dto.go`, etc.
- Register routes via `RouteRegistrar`.
- Put portable request constraints in OpenAPI and use `x-oapi-codegen-extra-tags` only for Go binding, normalization, and validator tags.
- Update `contracts/http/openapi.yaml`, run `make openapi-generate`, then adapt handlers to the generated request types.
- Keep handlers thin: bind/validate -> call usecase -> map response.
