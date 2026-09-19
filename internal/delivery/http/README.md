# HTTP delivery

This package adapts HTTP requests to application usecases and owns the Echo server boundary.

## Responsibilities and boundaries

- `server.go` owns Echo construction, injected binding and validation, middleware installation, route registration, and transport shutdown.
- Handlers bind and validate transport input, call usecases, and map results through the shared responder.
- OpenAPI-generated request and response types remain under `openapi/gen` and do not cross into usecases.
- `httpcontext` contains typed accessors for request state shared by handlers, responders, and middleware.

## Extending

- Update the OpenAPI contract and regenerate before adapting a handler.
- Add handlers under `handler/<feature>` and register routes through `RouteRegistrar`.
- Keep transport validation and response mapping here; keep business rules in usecases or domain types.
