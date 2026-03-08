# internal/domain

Pure domain layer.

## Pattern used

- Holds domain models and value objects only.
- Keeps domain rules framework-agnostic.

## How to extend

- Add entities/value objects/domain services with pure Go dependencies.
- Do not import infra, delivery, SQL, or framework packages.
