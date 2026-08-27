# internal/delivery/http/response

## Pattern used

- `Responder` gives returned context cancellation and deadline errors priority, then maps app errors to HTTP status.
- `SSEStream` owns response framing and flushing for long-lived Server-Sent Events.
- Response/error metadata for observability is stored through explicit `httpcontext` functions.
- The responder is stateless and is injected into handlers and middleware.

## How to extend

- Add transport-level response behavior as `Responder` methods.
- Open SSE streams after validation and keep one goroutine responsible for each stream's writes.
- For new Echo context values, add focused set/get functions in `httpcontext/`.
