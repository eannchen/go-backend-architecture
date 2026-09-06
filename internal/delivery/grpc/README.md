# internal/delivery/grpc

## Pattern used

- Generated protobuf and gRPC types live in `gen/` and remain transport-only.
- Service adapters under `service/` map generated messages to usecase inputs and outputs.
- Standard and custom gRPC services share the `service/` hierarchy; adapters for the official health protocol live in `service/health/`.
- The standard health reporter owns periodic readiness refreshes and its background lifecycle.
- The injectable responder under `response/` centralizes application-error to gRPC-status mapping.
- Interceptors provide shared request IDs, unary deadlines, recovery, tracing, access logs, and bounded-cardinality metrics.
- `server.go` owns TCP listening, injected transport credentials, service registration, optional reflection, interceptor chains, and graceful shutdown.
- Source contracts live in `contracts/grpc/` and are organized by versioned protobuf package.

## How to extend

- Update the `.proto` contract first, then run `make proto-lint proto-generate`.
- Add service implementations under `service/<feature>/`; never edit generated files.
- Register services through `ServiceRegistrar`; compose request context, observability, and recovery interceptors in that order in app wiring.
- Return errors through the shared responder so wire messages stay safe and mappings stay consistent.
- Keep generated protobuf types out of usecase and repository contracts.
- Test complete RPC behavior through generated clients under `integration/`.
- Build TLS credentials outside delivery and inject them through `ServerConfig`.
