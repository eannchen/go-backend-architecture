# internal/delivery/http/middleware/ratelimit

## Pattern used

- Maps an app-owned rate-limit decision to HTTP 429 responses and the standard `Retry-After` header.
- Delegates keys, algorithm choice, and failure policy to the usecase layer.

## How to extend

- Keep middleware transport-only; add a feature-specific limiter in its owning usecase.
- Use the shared response helper for rejection and technical errors.
