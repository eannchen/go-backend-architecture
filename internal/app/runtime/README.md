# Shared process runtime

This package owns resources and lifecycle behavior that are not specific to HTTP or gRPC.

## Responsibilities and boundaries

- `Telemetry` owns the logger and observability providers.
- `Runtime` owns process-shared configuration plus PostgreSQL and Redis resources.
- `RunLifecycle` coordinates blocking startup with root-context cancellation and bounded shutdown; each `cmd` package converts OS signals into that root context.

## Extending

- Add only resources whose construction and lifecycle are shared by multiple process types.
- Expose one shutdown path for every owned resource and preserve reverse dependency order.
- Keep handlers, services, workers, and feature wiring in their process composition packages.
