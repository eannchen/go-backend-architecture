# internal/app

## Pattern used

- `runtime` provides lightweight telemetry setup and the full configuration, DB, and Redis runtime used by servers.
- `httpapi` composes the HTTP API process from that runtime.
- `grpcapi` composes the standalone gRPC process from that runtime.
- `grpcclient` composes the one-shot service-to-service client demonstration without data-store dependencies.

## How to extend

- Add shared process dependencies in `runtime`; add future worker compositions in a sibling `internal/app/<process>/` package.
- Keep transport-specific wiring in `httpapi`, `grpcapi`, and `grpcclient`, with purpose-specific filenames.
- Keep business logic out of this package.
