# internal/infra/repoimpl

Infrastructure-only implementations and wrappers for repository contracts.

## Pattern used

- Adapter/decorator implementations that stay outside domain/usecase layers.
- Keeps repository contracts in `internal/repository` untouched.

## How to extend

- Add feature-focused implementation folders (for example `accountsummary/` or `orders/`).
- Keep behavior composition here (cached/traced/decorated variants).
- Keep vendor-specific clients/adapters in their own infra packages (`db/postgres`, `cache/redis`).
