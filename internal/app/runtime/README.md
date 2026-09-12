# internal/app/runtime

## Pattern used

- `Telemetry` owns the process logger and observability providers for processes that do not need data services.
- `Runtime` extends that foundation with configuration, database, and Redis connections for backend servers.
- Provides one shutdown boundary for those shared resources.
- Provides the common start, signal, and graceful-shutdown lifecycle used by process entrypoints.

## How to extend

- Add only process-neutral dependencies here.
- Keep API handlers, workers, and feature wiring in their process composition package.
