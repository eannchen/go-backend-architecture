---
description: Feature Structure
---

# Feature Structure

When adding a feature, create in order:

```
internal/usecase/<feature>/usecase.go
internal/repository/<feature>.go
internal/infra/db/postgres/store/<feature>.go
internal/delivery/http/<feature>/  (handler.go, dto.go)
```

Then wire in `internal/app/wiring.go`. Do not skip layers.

