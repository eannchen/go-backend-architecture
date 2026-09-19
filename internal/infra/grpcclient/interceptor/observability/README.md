# Outbound gRPC observability

This optional interceptor records client spans, completion logs, and metrics selected for one dependency.

## Responsibilities and boundaries

- Tracing, metrics, and logging are independently enabled around one shared RPC or stream outcome.
- Trace metadata is propagated only when configured for a trusted dependency.
- Completion logging uses dependency-specific level and skip policy rather than one global status classification.
- Standard fields describe RPC and destination facts; detailed remote messages stay out of metric dimensions.
- Stream telemetry finishes when `SendMsg` or `RecvMsg` reports that the stream ended; Go's normal `io.EOF` end-of-stream signal is recorded as success.

## Extending

- Put typed response interpretation and business telemetry in the remote dependency adapter.
- Add only bounded dependency, method, destination, call-shape, and status metric fields.
- Preserve stream wrapping when adding lifecycle behavior.
