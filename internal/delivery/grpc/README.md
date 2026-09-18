# gRPC delivery

This package adapts gRPC calls to application usecases and owns the gRPC server boundary.

## Responsibilities and boundaries

- Generated Protobuf types remain under `gen`; services map them to and from usecase types.
- `server.go` owns listening, server options, interceptor chains, service registration, reflection, and graceful transport shutdown.
- Interceptors provide request IDs and deadlines, caller identity, observability, and panic recovery.
- The responder maps application and delivery failures to safe gRPC statuses while preserving internal causes.
- Standard gRPC health and custom diagnostics are separate services with different contracts.

## Extending

- Change the Protobuf contract and regenerate before implementing a new RPC.
- Add the adapter under `service/<feature>` and register it through `ServiceRegistrar` in app wiring.
- Keep generated types and gRPC status mapping inside delivery.
- Exercise complete RPC behavior through generated clients in `integration` tests.
