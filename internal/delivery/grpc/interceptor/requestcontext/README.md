# internal/delivery/grpc/interceptor/requestcontext

## Pattern used

- Valid incoming request IDs are added to the context; accepting and returning request-ID metadata are separate, optional policies.
- Missing IDs are not generated because native trace IDs provide automatic correlation.
- Malformed incoming IDs are ignored by default so optional interoperability metadata cannot block business work; strict rejection is opt-in.
- Unary calls receive the configured server timeout while preserving any earlier client deadline.
- Streaming calls carry request metadata without a server-wide stream deadline.
- Handler errors pass through unchanged; each service is the final response-mapping boundary and should use the shared gRPC responder.

## How to extend

- Select metadata keys in app wiring when callers share a request-ID convention; leave them empty to rely only on tracing.
- Add protocol-level context values here only when every gRPC service should receive them.
- Keep authorization and rate-limit policy in separate interceptors.
- Keep application-error and context-error mapping in services rather than adding post-handler normalization here.
