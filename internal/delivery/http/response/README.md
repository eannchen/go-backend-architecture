# internal/delivery/http/response

## Pattern used

- `Responder` adapts payloads and maps app errors to HTTP status.
- Response/error metadata for observability is stored via `httpcontext.Meta`; this package forwards the type for `Responder`'s API.
- Constructor injection keeps handlers and middleware decoupled from Echo context keys.

## How to extend

- Add transport-level response behavior as `Responder` methods.
- For new Echo context values, add them in `httpcontext/` and use or forward types here as needed.
