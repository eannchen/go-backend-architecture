# cmd/grpcclient

## Pattern used

- Runs a one-shot trusted-internal-service client demonstration against `cmd/grpcapi`.
- Prints one JSON report after custom health, standard health, and streaming health calls complete.
- Starts and flushes its own logger and OpenTelemetry providers without connecting to database or Redis services.

## How to extend

- Keep connection and RPC composition in `internal/app/grpcclient`.
- Replace this demonstration command with a worker or backend process when adopting the client foundation.
