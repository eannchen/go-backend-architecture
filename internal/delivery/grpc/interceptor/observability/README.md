# internal/delivery/grpc/interceptor/observability

## Pattern used

- One public unary/stream interceptor coordinates separate tracing, access-log, and request-metrics components.
- Returned responder errors carry the original cause and safe gRPC status, so tracing and logging need no context side channel.
- OTel fields use the current RPC convention: `rpc.system.name`, a fully-qualified `rpc.method`, the native string `rpc.response.status_code`, and `error.type` only for statuses classified as server failures.
- Template-owned fields use an `app.*` prefix: call shape is `app.rpc.call_type`, while responder code, details, safe message, and internal cause chain use `app.error.*`.
- Metrics use only bounded RPC and native status fields; detailed application diagnostics stay in traces and logs.

## How to extend

- Add detailed diagnostic fields to tracing and access logs; add only stable method- or status-level metric fields.
- Keep standard OTel names for protocol facts and prefix template-specific fields with `app.` so ownership stays visible.
- Keep OpenTelemetry imports in infra and evolve shared contracts under `internal/observability`.
