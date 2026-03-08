# internal/service

Domain service abstractions and implementations.

## Pattern used

- Contains cross-aggregate/domain service logic when needed.
- Keeps service logic independent from transport and infra providers.

## How to extend

- Add services only when behavior does not naturally belong to a single entity/usecase.
- Keep dependencies on repository contracts or pure domain types, not infra implementations.
