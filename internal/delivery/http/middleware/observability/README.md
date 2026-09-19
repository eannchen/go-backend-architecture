# HTTP server observability

This middleware records server spans, access logs, and request metrics from one completed request outcome.

## Responsibilities and boundaries

- Calls the next handler once and shares its result across tracing, logging, and metrics.
- The responder stores the original error, safe response details, and application code in one typed `httpcontext` outcome.
- Attributes defined by OpenTelemetry keep their standard names; project-specific attributes use the `app.` prefix.
- Concrete paths, caller-specific values, and detailed errors stay out of metric dimensions.

## Extending

- Add detailed diagnostics only to traces and logs.
- Add metric attributes only when their value set is bounded.
- Keep response interpretation centralized so all three signals report the same outcome.
