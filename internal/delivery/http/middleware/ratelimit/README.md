# HTTP rate-limit middleware

This middleware maps an application rate-limit decision to an HTTP response.

## Responsibilities and boundaries

- Returns HTTP 429 and, when available, the HTTP `Retry-After` response header.
- Uses Echo's resolved client IP as the global limiter key.
- Delegates quota configuration, the token-bucket operation, and dependency-failure behavior to the usecase.

## Extending

- Add feature-specific policies to the usecase that owns the protected operation.
- Keep this middleware limited to HTTP input and response mapping.
