# internal/usecase/globalratelimit

## Pattern used

- Owns the global per-IP policy and key shape; the Redis algorithm remains behind a repository contract.
- Fails closed when the client IP or Redis limiter is unavailable.

## How to extend

- Keep transport mapping in middleware and add feature-specific limits in the owning usecase.
- Use the token bucket for burst-tolerant traffic; use the sliding-window contract for strict rolling limits.
