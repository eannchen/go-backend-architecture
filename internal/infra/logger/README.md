# internal/infra/logger

## Pattern used

- Contract (interface + types) in `internal/logger`; this package provides the implementation.
- Structured key-value logging; no vendor types leak across boundary.

## How to extend

- Extend contract in `internal/logger` when app layers need new capability.
- Add/change implementation here; wire in app.
