# internal/app/runtime

## Pattern used

- Owns dependencies shared by every deployable process: configuration, logging, database pool, and observability.
- Provides one shutdown boundary for those shared resources.

## How to extend

- Add only process-neutral dependencies here.
- Keep API handlers, workers, and feature wiring in their process composition package.
