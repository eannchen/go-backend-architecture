---
description: Architecture Layers
---

# Architecture Layers

Dependency direction:

```
delivery -> usecase -> repository contracts
infra -> repository contracts
app -> wires everything together
```

### delivery

Transport only: HTTP handlers, request validation, response mapping. No business logic.

### usecase

Business logic: orchestration, domain rules, calling repositories. Independent of frameworks and infrastructure.

### repository (contracts)

Interfaces used by usecases (e.g. `AccountRepository`, `TxManager`, `CacheStore`). No database code here.

### infra

Implements repository contracts: postgres, redis, logger, observability, external services. Depends on contracts only.

### app

Composition root: dependency wiring, adapters between packages, starting the server. Cross-package wiring happens here only.

