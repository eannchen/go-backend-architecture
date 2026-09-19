# Observability contracts

This package defines vendor-neutral tracing, metrics, log-emission, and propagation behavior shared across layers.

## Responsibilities and boundaries

- Delivery and infrastructure instrumentation depend on these contracts instead of importing OpenTelemetry.
- Carrier interfaces support server extraction and client injection without exposing transport types.
- Request IDs use explicit context helpers; trace identity remains in the native active span context.
- Shared error formatting keeps tracing and logging outcomes consistent.

## Extending

- Add a contract operation when code outside the OpenTelemetry implementation needs that behavior.
- Implement vendor behavior under `internal/infra/observability` and wire lifecycle through app runtime.
- Keep transport field selection in the owning middleware or interceptor.
