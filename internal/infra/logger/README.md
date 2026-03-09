# internal/infra/logger

Logger implementation(s). Contract lives in `internal/logger`; app layers depend on the contract only.

## Pattern used

- Contract (interface + types) in a separate package; this package provides the implementation.
- Structured key-value logging and optional secondary sink; no vendor types leak across the boundary.
- Single implementation subpackage; additional backends go in further subpackages.

## How to extend

- Extend the contract in `internal/logger` when app layers need new capability.
- Add or change implementation here; keep vendor and config types inside infra.
- Wire the implementation in app; do not expose vendor types to usecase or delivery.
