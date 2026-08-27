# internal/delivery/grpc/interceptor/requestcontext

## Pattern used

- Unary and stream interceptors validate or generate `x-request-id`, place it in context, and return it as response metadata.
- Unary calls receive the configured server timeout while preserving any earlier client deadline.
- Streaming calls carry request metadata without a server-wide stream deadline.
- Handler errors pass through unchanged; each service is the final response-mapping boundary and should use the shared gRPC responder.

## How to extend

- Add protocol-level context values here only when every gRPC service should receive them.
- Keep authorization and rate-limit policy in separate interceptors.
- Keep application-error and context-error mapping in services rather than adding post-handler normalization here.
