# internal/app

## Pattern used

- `runtime` owns shared process setup: configuration, logging, DB pool, and observability.
- `api` composes the HTTP API process from that runtime.

## How to extend

- Add shared process dependencies in `runtime`; add future worker compositions in a sibling `internal/app/<process>/` package.
- Keep API-specific wiring in `api` and name its files `api_*.go`.
- Keep business logic out of this package.
