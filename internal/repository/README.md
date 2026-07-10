# internal/repository

## Pattern used

- Contracts (interfaces) for usecases. Subdirs mirror `internal/infra` layout: `db/`, `cache/`, `kvstore/`, `external/`.
- Files named `xxxx_repository.go` (e.g. `health_repository.go`, `tx_manager_repository.go`).
- Contracts describe behavior, not storage technology. No driver/ORM types in interfaces.
- Repository boundaries are usecase/consumer-driven, not schema/table-driven.
- One schema/table does not imply one repository interface. Group methods by business capability.
- Transactions controlled by usecase via `db/TxManager`.
- Struct ownership: repository packages define their own structs — never import from `internal/domain`. Domain types belong to usecases. Structs may carry raw storage values and DB-computed aggregations.

## How to extend

- Add interface in the subdir matching the infra area (`db/`, `cache/`, `kvstore/`, `external/`).
- Use `xxxx_repository.go` naming. Keep methods minimal and business-driven.
- Prefer capability names (`user_repository.go`, `auth_repository.go`) over schema names.
- Avoid ORM-style generic CRUD repositories that mirror tables directly; expose only operations needed by usecases.
- Implement under the corresponding `internal/infra/...` package.
