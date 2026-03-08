# internal/repository

Repository contracts (interfaces) owned by usecases.

## Pattern used

- Contracts describe behavior, not storage technology.
- Keep driver/ORM/sql types out of interfaces.
- Keep transactions controlled by usecase via tx manager contract.

## How to extend

- Add new interface for each aggregate/persistence boundary.
- Keep methods minimal and business-driven.
- Implement interfaces under `internal/infra/...`.
