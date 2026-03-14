# internal/domain

## Pattern used

- Pure domain models and value objects. Framework-agnostic.

## How to extend

- Add entities, value objects, or domain services with pure Go dependencies.
- Do not import infra, delivery, SQL, or framework packages.
