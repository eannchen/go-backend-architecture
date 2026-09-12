# internal/observability/grpcsemconv

## Pattern used

- Centralizes the gRPC values required by current OpenTelemetry RPC semantic conventions.
- Keeps client and server failure classification explicit because the same status can have different meaning on each side.
- Contains no tracing SDK or interceptor behavior.

## How to extend

- Add only gRPC-to-OTel normalization shared by client and server instrumentation.
- Keep request models, destination parsing, logging policy, and interceptor lifecycle in their owning packages.
