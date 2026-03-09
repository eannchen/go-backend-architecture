# internal/observability

Observability contracts (Tracer, Span, Meter, LogEmitter, Runtime) used by usecase, delivery, and infra. Implementations live in infra.

## Pattern used

- Framework-agnostic interfaces so app layers do not import OpenTelemetry.
- Context helpers (`WithRequestID`, `WithTrace`, etc.) carry correlation IDs across layers.
- No infra import in usecase/delivery for observability; only this package.

## How to extend

- Add new capabilities behind interfaces first.
- Keep OTel/vendor details inside `internal/infra/observability/otel`.
- Wire implementations in app; ensure shutdown is propagated.
