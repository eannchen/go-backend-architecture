# internal/infra

## Pattern used

- Adapter pattern: each subpackage implements one kind of contract (persistence, cache, KV, external service, logging, observability).
- Vendor/SDK details stay inside infra. Only contract-friendly types cross the boundary.
- Returns standard Go errors (`fmt.Errorf` with `%w`); usecases wrap into `apperr`.

## How to extend

- Add a subpackage per infrastructure concern; implement the contract from `internal/repository` or the relevant contract package.
- Keep constructors explicit and injectable; wire in `internal/app` only.
