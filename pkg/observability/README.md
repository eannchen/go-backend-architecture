# Observability contracts

This library defines vendor-neutral tracing, metrics, log-emission, and propagation contracts, with an OpenTelemetry implementation under `otel`. It imports no project `internal` packages.

## Responsibilities and boundaries

- Delivery and infrastructure instrumentation depend on these contracts instead of importing OpenTelemetry.
- Carrier interfaces support server extraction and client injection without exposing transport types.
- Request IDs use explicit context helpers; trace identity remains in the native active span context.
- Shared error formatting keeps tracing and logging outcomes consistent.
- Inject non-nil contracts. Use `NoopTracer`, `NoopMeter`, or `NoopRuntime` when a capability is disabled.

## Extending

- Add a contract operation when code outside the OpenTelemetry implementation needs that behavior.
- Implement vendor behavior under `pkg/observability/otel` and wire lifecycle through app runtime.
- Keep transport field selection in the owning middleware or interceptor.

## Lifecycle and integration

- Applications load environment values and pass `otel.Config` plus service identity to `otel.Setup`.
- `Setup` installs process-wide OTel providers and W3C propagation. Call once during startup; disabling export keeps local trace IDs and propagation active.
- Runtime provider getters allow SDK hooks such as `KafkaHooks` to use those same providers. Stop clients and processing before calling `Runtime.Shutdown` with a bounded cleanup context.
- `observabilitytest` contains reusable test doubles; `grpcsemconv` contains shared gRPC telemetry conventions.
