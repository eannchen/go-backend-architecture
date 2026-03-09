---
description: Dependency Rules
---

# Dependency Rules

**Allowed:** `delivery -> usecase`, `usecase -> repository`, `infra -> repository`, `app -> all layers`.

**Forbidden:** usecase must NOT import `internal/infra` or delivery; repository must NOT import infra; domain must NOT import outer layers. Only `internal/app` may import across layers.

