# internal/delivery/http/response

HTTP response writer and response metadata contracts.

## Pattern used

- Adapter object (`Responder`) for writing client payloads and mapping app errors to HTTP status.
- Metadata contract (`Meta`) records response and error context for observability middleware.
- Constructor injection keeps handlers and middleware decoupled from Echo context key details.

## How to extend

- Add new response writing behavior as `Responder` methods when it is transport-level and reusable.
- Keep metadata keys and read/write behavior in `Meta` implementations.
- Reuse `NewContextMeta()` in middleware and handlers to keep metadata semantics consistent.
