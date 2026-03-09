---
description: HTTP Observability
---

# HTTP Observability

All routes register via `RouteRegistrar`. Middleware provides request-level tracing. Add spans in handlers, usecases, and repositories where important. Do not import OpenTelemetry outside observability packages; use observability interfaces.

