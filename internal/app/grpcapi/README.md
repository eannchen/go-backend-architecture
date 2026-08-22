# internal/app/grpcapi

## Pattern used

- Composes the standalone gRPC process from shared runtime dependencies.
- Builds health repositories and usecase, custom diagnostics, standard health, and the transport server.
- Starts and shuts down the standard health reporter around the transport server lifecycle.

## How to extend

- Add gRPC-specific constructor wiring in the matching `grpcapi_*_wiring.go` file.
- Register generated services through delivery's service registrar.
- Keep process-neutral resources in internal/app/runtime and protocol mapping in internal/delivery/grpc.
