---
description: Architecture Layers
---

# Architecture Layers

```
delivery -> usecase -> repository contracts
infra -> repository contracts
app -> wires everything together
```

- **delivery** — Transport only: handlers, validation, response mapping.
- **usecase** — Business logic, independent of frameworks.
- **repository** — Contracts (interfaces) for usecases. Subdirs mirror infra: `db/`, `cache/`, `kvstore/`, `external/`.
- **infra** — Implements contracts: postgres, redis, external services, logger, observability. `composed/` holds decorator stores that combine multiple implementations (e.g. cache-aside).
- **app** — Composition root: wiring, adapters, server startup.

