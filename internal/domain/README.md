# internal/domain

## Pattern used

- Pure business entities, value objects, and domain services. Framework-agnostic.
- Usecases and repository contracts may depend on domain types; domain never depends on those outer layers.
- The current `user` entity is shared across authentication usecases and persistence contracts without exposing storage models.

## How to extend

- Add a domain type when it represents business meaning shared across workflows or owns an invariant.
- Let infrastructure map database and provider models to domain types at repository boundaries.
- Keep workflow orchestration, I/O, transport validation, and application error mapping outside domain.
- Do not import infra, delivery, SQL, or framework packages.
