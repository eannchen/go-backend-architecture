---
description: Feature Structure
---

# Feature Structure

Create in order, then wire in `internal/app/wiring.go`:

```
internal/usecase/<feature>/
internal/repository/<area>/<feature>_repository.go
internal/infra/<area>/store/<feature>_store.go
internal/delivery/http/handler/<feature>/
```

