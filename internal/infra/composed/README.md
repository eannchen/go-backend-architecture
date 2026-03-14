# internal/infra/composed

## Pattern used

- Decorator pattern: each subpackage composes multiple infra implementations behind one repository contract (e.g. cache-aside wraps a DB store with a cache store, exposing the same `UserRepository` interface).
- The composed store owns coordination logic (cache hit/miss, invalidation) so usecases stay unaware of caching.
- Files named `<feature>_cached_store.go`; each subpackage is one feature (e.g. `user/`).

## How to extend

- Add a subpackage per feature that needs composition (e.g. `composed/product/`).
- Implement the same repository contract as the base store. Accept the base + cache (or other layer) via constructor injection.
- Wire in `internal/app/wiring.go` by wrapping the base store with the composed store.
