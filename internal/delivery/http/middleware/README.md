# internal/delivery/http/middleware

Transport-level middleware.

## Pattern used

- Subdirectory-per-middleware design for clear boundaries.
- `context/` provides request-id and timeout propagation middleware.
- `observability/` provides tracing and request logging middleware.
- Shared `response.Meta` centralizes response metadata access.

## How to extend

- Add middleware only for cross-cutting behavior (auth, tracing, rate limit, request-id).
- Create a new middleware subdirectory and keep one middleware concern per package.
- Prefer struct components with constructor injection and a `Handler() echo.MiddlewareFunc` adapter.
- Reuse shared response metadata from `internal/delivery/http/response` instead of adding new context keys in components.
- Avoid business decisions or repository/usecase calls inside middleware.
