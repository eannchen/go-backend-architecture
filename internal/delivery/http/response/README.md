# HTTP responses

This package writes consistent HTTP success, error, and server-sent event responses.

## Responsibilities and boundaries

- The responder prioritizes request cancellation and deadline failures, then maps application errors to HTTP status and safe JSON payloads.
- Error outcome metadata is stored through typed `httpcontext` accessors for observability.
- `SSEStream` owns event framing and flushing after the handler has validated the request.

## Extending

- Add shared transport behavior to the responder rather than duplicating it in handlers.
- Add new context state through focused `httpcontext` accessors.
- Keep one goroutine responsible for writes to each SSE stream.
