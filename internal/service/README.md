# internal/service

## Pattern used

- Cross-aggregate domain service logic when behavior does not belong to a single usecase.
- Independent from transport and infra.

## How to extend

- Add services only for cross-aggregate behavior.
- Depend on repository contracts or domain types, not infra.
