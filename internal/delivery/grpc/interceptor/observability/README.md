# gRPC server observability

This interceptor records server spans, access logs, and request metrics from one completed RPC outcome.

## Responsibilities and boundaries

- Unary and stream interceptors coordinate tracing, logging, and metrics without changing service responses.
- Responder errors retain both the original cause and safe gRPC status, so error details do not need to be stored separately in context.
- Attribute names beginning with `rpc.` and the `error.type` attribute follow OpenTelemetry conventions; names beginning with `app.` are defined by this project.
- Detailed errors and caller identity stay in traces and logs; metric dimensions remain bounded.

## Extending

- Add detailed diagnostic fields only to traces and logs.
- Add a metric attribute only when its value set is bounded and useful for aggregation.
- Keep OpenTelemetry imports in `internal/infra/observability` and shared conventions in `internal/observability`.
