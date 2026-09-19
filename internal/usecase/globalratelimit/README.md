# Global rate-limit usecase

This package decides whether public HTTP traffic is allowed by the global per-IP policy.

## Responsibilities and boundaries

- Denies the request when the client IP is missing or the limiter is unavailable instead of allowing unmetered traffic.
- Calls the token-bucket repository; Redis commands and HTTP response details stay outside the usecase.

## Extending

- Add feature-specific limits to the usecase that owns the protected operation.
- Choose token-bucket behavior for controlled bursts and sliding-window behavior for strict rolling limits.
- Keep HTTP status and the `Retry-After` response header in delivery middleware.
