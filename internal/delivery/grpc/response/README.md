# internal/delivery/grpc/response

## Pattern used

- An injectable responder maps application errors and delivery validation failures to gRPC statuses.
- Returned errors expose safe client messages while retaining their original causes for observability.

## How to extend

- Add new application-code mappings centrally when `apperr` gains a code.
- Keep transport-specific validation messages at the service call site.
- Add protobuf status details only when clients have a documented contract for them.
