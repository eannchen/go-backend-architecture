# internal/infra/cache

Cache implementation(s). Implements cache-related repository contracts; contracts live in `internal/repository`.

## Pattern used

- One implementation subpackage per backend; each subpackage owns connection and store logic.
- Serialization and key policy stay in this layer; usecases depend only on repository interfaces.
- Store types implement narrow contracts (e.g. read-model cache, health) rather than a single generic cache.

## How to extend

- Add a new subpackage per backend, implementing the same repository contracts.
- Keep interfaces small and focused in `internal/repository`; add new store files under the backend subpackage for new capabilities.
- Do not push serialization or backend-specific policy into usecase.
