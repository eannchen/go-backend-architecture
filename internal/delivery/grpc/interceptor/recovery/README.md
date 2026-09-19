# gRPC panic recovery

This interceptor converts panics from RPC handling into safe internal gRPC failures.

## Responsibilities and boundaries

- Logs the panic and stack internally without exposing them to clients.
- Wraps unary and streaming handlers.
- Runs inside observability so a recovered panic is recorded as the RPC outcome.

## Extending

- Preserve gRPC's `Internal` status code and its client-safe message.
- Keep recovery inside observability in the interceptor chain.
