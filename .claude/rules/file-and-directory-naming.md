---
description: File and directory naming
---

# File and directory naming

Names should make **purpose visible from the editor tab**.

- **Handlers:** `handler/<feature>/` with `<feature>_<role>.go` (e.g. `auth_handler.go`, `auth_dto.go`, `health_handler.go`).
- **Middleware:** `<feature>_middleware.go`, `<feature>_<specific>_middleware.go`; support files without `_middleware` (e.g. `observability_keys.go`); tests `<feature>_middleware_test.go`.
- **Usecase:** `<feature>_usecase.go` with interface + impl in one file. Multi-capability features use subdirs (e.g. `auth/otp/otp_usecase.go`); shared types in the parent (`auth_types.go`).
- **Repository:** `xxxx_repository.go` in the matching subdir (`db/`, `cache/`, `kvstore/`, `external/`).

