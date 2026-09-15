# internal/delivery/grpc/interceptor/recovery

## Pattern used

- Unary and stream interceptors recover panics at the transport boundary.
- The original panic and stack are logged internally; clients receive a safe `Internal` status.

## How to extend

- Keep recovery as the innermost infrastructure interceptor so outer observability sees the mapped failure.
- Do not expose panic values or stack traces in gRPC status messages.
