# internal/infra/config

Configuration loading and validation. Used at startup by app and infra; no contract package (config is infra-only).

## Pattern used

- Environment and defaults are parsed once into typed structs; validation runs at bootstrap.
- Config types are consumed by app wiring and infra constructors; they are not exposed to usecase or delivery as-is.

## How to extend

- Add or change fields on the config structs, then update env parsing and validation in one place.
- Keep defaults and fallbacks explicit and deterministic; avoid implicit or global overrides.
