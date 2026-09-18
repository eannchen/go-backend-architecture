# gRPC telemetry conventions

This package normalizes gRPC values shared by client and server instrumentation.

## Responsibilities and boundaries

- Removes gRPC's wire-path prefix from method names and returns canonical names for status codes.
- Classifies client and server failures separately because one status can have different meaning on each side.
- Contains no tracing SDK, logging policy, destination parsing, or interceptor lifecycle.

## Extending

- Add only gRPC-to-telemetry normalization used by both inbound and outbound instrumentation.
- Keep application fields and signal-specific behavior in their owning middleware or interceptor.
