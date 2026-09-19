# gRPC application composition

This package assembles and runs the standalone gRPC process.

## Responsibilities and boundaries

- Builds health dependencies, diagnostics and standard-health services, interceptors, credentials, and the transport server.
- Owns interceptor order and the standard health reporter's start and shutdown sequence.
- Wraps the standard-library `tls.Config` as gRPC credentials; certificate loading remains in infrastructure.

## Extending

- Add constructor wiring to the matching `grpcapi_*_wiring.go` file.
- Register generated services through delivery's `ServiceRegistrar`.
- Keep shared process resources in `internal/app/runtime` and request or response mapping in `internal/delivery/grpc`.
