# internal/delivery/http/middleware/observability

## Pattern used

- One middleware owns the request lifecycle and calls the next handler once.
- Separate tracing, access-log, and request-metrics components consume one normalized request outcome.
- The responder records one `httpcontext.ErrorOutcome` containing the original error, application/delivery code, safe message, and diagnostic details.
- Standard OTel fields describe the HTTP method, route, concrete path, scheme, response status, and server failure type.
- Template-owned responder fields use `app.error.*`, keeping application codes and safe response details distinct from native HTTP status.
- Metrics use bounded method, route, and status fields only; the concrete path and detailed diagnostics stay in traces and logs.

## How to extend

- Add detailed diagnostic fields to tracing and access logs; add only bounded fields to metrics.
- Keep standard OTel names for protocol facts and prefix template-specific fields with `app.` so ownership stays visible.
- Keep response interpretation in the shared outcome so all three components observe the same result.
