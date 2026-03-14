# internal/observability

## Pattern used

- Framework-agnostic interfaces (Tracer, Span, Meter, LogEmitter) so app layers do not import OpenTelemetry.
- Context helpers carry correlation IDs across layers.

## How to extend

- Add capabilities behind interfaces first.
- Keep OTel/vendor details in `internal/infra/observability/`.
- Wire implementations in app; ensure shutdown is propagated.
