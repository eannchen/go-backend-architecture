# Outbound gRPC connections

This package provides reusable gRPC connection mechanics without defining a universal remote-service client.

## Responsibilities and boundaries

- Owns targets, transport credentials, message limits, explicitly selected interceptors, and connection shutdown.
- `New` creates a lazy connection and does not wait for the remote server; use RPC deadlines where startup or readiness depends on a call.
- It does not assume shared deadlines, tracing, request IDs, authentication, logging, or metrics for every dependency.

## Extending

- Implement each remote dependency under `internal/infra/external` against its repository contract.
- Select connection-wide interceptors according to that dependency's trust and telemetry policy.
- Keep method deadlines, retries, response interpretation, error mapping, and business telemetry in the dependency adapter.
