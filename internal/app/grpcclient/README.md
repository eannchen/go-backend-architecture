# internal/app/grpcclient

## Pattern used

- Runs as its own lightweight process with logging and OpenTelemetry, without opening database or Redis connections.
- Composes the policy-neutral outbound connection with plaintext, TLS, or mTLS credentials.
- Treats `cmd/grpcapi` as a trusted internal dependency, so the demo explicitly opts into a call deadline, tracing, metrics, a demo-specific completion-log policy, trace propagation, and propagation of an existing request ID.
- Demonstrates custom unary diagnostics, standard unary health, and standard server-streaming health on one shared connection.
- Returns protocol-neutral report data to the command entrypoint and shuts down the connection and telemetry explicitly.

## How to extend

- Use this app only as a learning example. For a real dependency, define a usecase-facing contract in `internal/repository/external/` and implement it in a service-specific `internal/infra/external/` adapter.
- Construct generated clients from `Client.Connection()`, then keep protobuf mapping and provider-specific policies inside that adapter.
- Disable trace and request-ID propagation, or omit those interceptors entirely, when a dependency does not share the internal metadata contract.
