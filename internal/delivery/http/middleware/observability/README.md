# internal/delivery/http/middleware/observability

## Pattern used

- One middleware owns the request lifecycle and calls the next handler once.
- Separate tracing, access-log, and request-metrics components consume one normalized request outcome.
- The responder records the original error, application/delivery code, and safe response message in `httpcontext`.
- Protocol status is recorded separately from application error metadata; metrics use bounded method, route, and status fields only.

## How to extend

- Add detailed diagnostic fields to tracing and access logs; add only bounded fields to metrics.
- Keep response interpretation in the shared outcome so all three components observe the same result.
