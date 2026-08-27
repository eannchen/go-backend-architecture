# internal/delivery/grpc/interceptor/observability

## Pattern used

- One public unary/stream interceptor coordinates separate tracing, access-log, and request-metrics components.
- Returned responder errors carry the original cause and safe gRPC status, so tracing and logging need no context side channel.
- Traces and logs record the original error chain, application code/details, and safe response message.
- `rpc.grpc.status_code` records the protocol status separately; metrics use bounded RPC and status fields only.

## How to extend

- Add detailed diagnostic fields to tracing and access logs; add only stable method- or status-level metric fields.
- Keep OpenTelemetry imports in infra and evolve shared contracts under `internal/observability`.
