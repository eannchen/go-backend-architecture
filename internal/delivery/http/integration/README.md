# HTTP integration tests

This package verifies HTTP feature workflows through real handlers, usecases, repositories, PostgreSQL, and Redis.

## Responsibilities and boundaries

- The server fixture owns common HTTP mechanics, binder, validation, and route registration.
- Feature fixtures own dependency wiring and cleanup; flow tests describe client-visible scenarios.
- PostgreSQL and Redis are disposable real backends, while unrelated external providers remain test implementations.
- SQL, Redis keys, serialization, expiration, and atomicity details are tested in their adapter packages instead.

## Extending

- Keep each feature's wiring and cleanup in its fixture.
- Reuse the server fixture and keep multi-step workflows explicit.
- Register cleanup for every created row and key.
