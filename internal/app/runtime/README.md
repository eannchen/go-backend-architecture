# internal/app/runtime

## Pattern used

- Owns dependencies shared by every deployable process: configuration, logging, database and Redis connections, and observability.
- Provides one shutdown boundary for those shared resources.
- Provides the common start, signal, and graceful-shutdown lifecycle used by process entrypoints.

## How to extend

- Add only process-neutral dependencies here.
- Keep API handlers, workers, and feature wiring in their process composition package.
