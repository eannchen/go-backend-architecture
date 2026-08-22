# internal/app

## Pattern used

- `runtime` owns shared process setup: configuration, logging, DB pool, and observability.
- `httpapi` composes the HTTP API process from that runtime.
- `grpcapi` composes the standalone gRPC process from that runtime.

## How to extend

- Add shared process dependencies in `runtime`; add future worker compositions in a sibling `internal/app/<process>/` package.
- Keep transport-specific wiring in `httpapi` and `grpcapi`, with matching `httpapi_*` and `grpcapi_*` filenames.
- Keep business logic out of this package.
