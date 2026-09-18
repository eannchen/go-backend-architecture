# HTTP panic recovery

This middleware converts panics from downstream HTTP handling into safe transport failures.

## Responsibilities and boundaries

- Logs panic details internally without exposing them to clients.
- Uses the shared responder only when the response has not been committed.
- Never overwrites a committed response. It rethrows Go's `http.ErrAbortHandler` sentinel so the HTTP server can abort the connection without writing another response.
- Runs inside observability so recovered panics are recorded as request outcomes.

## Extending

- Keep panic values and stacks out of response payloads.
- Add special handling only when required by Echo or `net/http` behavior.
