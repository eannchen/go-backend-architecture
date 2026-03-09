# internal/infra/repoimpl

Composite repository implementations (e.g. cached, decorated). Wraps store implementations; repository contracts stay in `internal/repository`.

## Pattern used

- Composes or decorates concrete store implementations to add cross-cutting behavior (caching, tracing) without changing contracts.
- Feature-focused subpackages; each implements one or more repository interfaces by delegating to infra stores.
- Vendor and store details stay in their own infra packages; this package only composes and delegates.

## How to extend

- Add a feature-focused subpackage when you need a composed or decorated implementation of existing contracts.
- Keep behavior composition here; keep store and vendor code in `db`, `cache`, etc.
- Do not add new repository contracts here; define contracts in `internal/repository` and implement in the appropriate infra package.
