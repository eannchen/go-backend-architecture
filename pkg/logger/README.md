# Structured logging

This library defines structured logging contracts, field helpers, a Zap implementation,
and configurable test doubles. It can be imported without the application's internal packages.

## Responsibilities and boundaries

- `logger.Logger` accepts contexts, messages, and one optional fields map per call.
- `zaplogger.New` returns a concrete logger configured with `zaplogger.Config`. `Level` controls
  local output; `SinkLevel` independently controls forwarding to the secondary sink.
- Applications load environment settings and attach their own context-field and sink callbacks.
- `loggertest` provides configurable doubles for tests that consume the logging contract.

## Lifecycle and extension

- Configure callbacks before concurrent logging begins. Passing nil disables an optional callback.
- Call `Sync` during shutdown after logging work has stopped, and handle its returned error.
- Add another implementation only when needed; keep application telemetry integration in app composition.
