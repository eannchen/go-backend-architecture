# Outbound gRPC request context

This interceptor applies optional default deadlines and request-ID propagation to outbound calls.

## Responsibilities and boundaries

- Applies the configured timeout without extending an earlier caller deadline.
- Propagates an existing request ID only when a metadata key is explicitly configured.
- Does not generate request IDs, add credentials, or retry RPCs.

## Extending

- Opt into the downstream service's documented metadata convention.
- Add only bounded context values that apply to every call on the connection.
- Keep authentication in a dedicated interceptor and retry policy beside the called operation.
