# internal/delivery/http/middleware/recovery

## Pattern used

- Recovery is a transport safety boundary for panics that handlers cannot map themselves.
- Uncommitted responses use the shared responder so clients receive the standard internal-error payload.
- Committed responses are never overwritten; the panic is recorded for observability and returned up the middleware chain.

## How to extend

- Keep panic values and stacks in internal logs only.
- Preserve `http.ErrAbortHandler` so `net/http` can terminate the request using its standard behavior.
