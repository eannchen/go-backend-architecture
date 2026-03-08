# internal/delivery/middleware

Transport-level middleware.

## Pattern used

- Chain of responsibility for cross-cutting transport concerns.
- Handles request context enrichment, tracing boundaries, and generic request policies.
- Must stay transport-focused and domain-agnostic.

## How to extend

- Add middleware only for cross-cutting behavior (auth, tracing, rate limit, request-id).
- Keep middleware reusable and configurable through constructor params.
- Avoid business decisions or repository/usecase calls inside middleware.
