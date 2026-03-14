# internal/infra/config

## Pattern used

- Environment and defaults parsed once into typed structs with validation at bootstrap.
- Consumed by app wiring and infra constructors; not exposed to usecase or delivery as-is.

## How to extend

- Add/change fields on config structs; update env parsing and validation in one place.
