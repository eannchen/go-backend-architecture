---
description: Error Handling
---

# Error Handling

Usecases return application errors from `internal/apperr` (`apperr.New`, `apperr.Wrap`); handlers convert them to transport responses. Infra returns standard Go errors (`fmt.Errorf` with `%w`); usecases wrap infra errors into app errors at the boundary. Handlers must NOT return raw errors.

