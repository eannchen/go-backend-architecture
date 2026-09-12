# internal/observability

## Pattern used

- Framework-agnostic interfaces (Tracer, Span, Meter, LogEmitter) so app layers do not import OpenTelemetry.
- Tracing supports server extraction and client injection without exposing transport or OTel carrier types.
- Context helpers carry request IDs across layers; trace identity remains in the active native span context.
- Shared error-chain formatting keeps transport tracing and logging consistent.

## How to extend

- Add capabilities behind interfaces first.
- Keep OTel/vendor details in `internal/infra/observability/`.
- Wire implementations in app; ensure shutdown is propagated.
