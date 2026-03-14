# internal/delivery/http/response

## Pattern used

- `Responder` adapts payloads and maps app errors to HTTP status.
- `Meta` records response/error context for observability middleware.
- Constructor injection keeps handlers and middleware decoupled from Echo context keys.

## How to extend

- Add transport-level response behavior as `Responder` methods.
- Reuse `NewContextMeta()` in middleware and handlers for consistent metadata.
