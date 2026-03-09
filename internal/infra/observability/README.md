# internal/infra/observability

Observability implementation. Contracts live in `internal/observability`; app layers depend on the contract package only.

## Pattern used

- Contract (tracing and log emission) in a separate package; this package provides the implementation.
- All vendor and SDK details stay inside this package and its subpackages; context helpers and no-ops live in the contract package.
- Lifecycle (startup/shutdown) is part of the implementation; app wiring propagates errors.

## How to extend

- Extend the contract in `internal/observability` when app or infra needs new capability.
- Add or change implementation here; keep vendor and SDK types inside infra.
- Ensure lifecycle hooks are invoked from app wiring and that startup/shutdown errors are propagated.
