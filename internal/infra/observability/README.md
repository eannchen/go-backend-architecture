# internal/infra/observability

Observability contracts and OTel implementation.

## Pattern used

- Contract + implementation split (`observability` vs `observability/otel`).
- `Tracer` and `LogEmitter` are framework-agnostic interfaces.
- Context helpers carry request/trace correlation across layers.

## How to extend

- Add new telemetry capabilities behind contracts first.
- Keep OTel/vendor details inside `otel/`.
- Ensure shutdown and startup errors are propagated to app wiring.
