# internal/delivery/http/middleware

## Pattern used

- Subdirectory-per-middleware for clear boundaries.
- Files named `<feature>_middleware.go` / `<feature>_<specific>_middleware.go`; support files without `_middleware` (e.g. `observability_keys.go`); tests `<feature>_middleware_test.go`.

## How to extend

- Add a new subdirectory per middleware concern.
- Use `xxx_middleware` file naming pattern.
- Prefer struct with constructor injection and `Handler() echo.MiddlewareFunc` adapter.
- Inject `Responder` from `response/` for consistent error format.
- Keep business logic out of middleware.
