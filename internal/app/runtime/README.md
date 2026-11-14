# Shared process runtime

This package owns resources and lifecycle behavior that are shared by the executable processes present in the project.

## Responsibilities and boundaries

- `Telemetry` owns the logger and observability providers.
- `Runtime` adds shared configuration, PostgreSQL, and Redis to `Telemetry`. Processes that do not need data services can use `Telemetry` directly.
- `RunLifecycle` coordinates blocking startup with root-context cancellation and bounded shutdown; each `cmd` package converts OS signals into that root context.

## Extending

- Add only resources whose construction and lifecycle are shared by multiple process types.
- Expose one shutdown path for every owned resource and preserve reverse dependency order.
- Keep handlers, services, workers, and feature wiring in their process composition packages.
