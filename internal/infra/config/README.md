# internal/infra/config

Configuration loading and validation.

## Pattern used

- Parses environment configuration into typed structs.
- Validates runtime config once during startup.

## How to extend

- Add new config fields to the typed struct first, then add env parsing and validation.
- Keep fallback/default behavior explicit and deterministic.
