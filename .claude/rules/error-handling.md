---
description: Error Handling
---

# Error Handling

Use application errors from `internal/apperr` (`apperr.New`, `apperr.Wrap`). Usecases return app errors; handlers convert them to transport responses. Handlers must NOT return raw errors. Wrap infrastructure errors before leaving infra.

