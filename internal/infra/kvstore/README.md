# internal/infra/kvstore

Key-value store implementation(s). Implements KV-related repository contracts; contracts live in `internal/repository`.

## Pattern used

- One implementation subpackage per backend; each subpackage owns connection and store logic.
- Backend usage and key semantics stay in this layer; usecases depend only on repository interfaces.
- Contract names reflect business semantics, not a generic key-value API.

## How to extend

- Add a new subpackage per backend, implementing the same repository contracts.
- Keep each concrete capability in its own store under the backend subpackage for clear structure.
- Do not push backend-specific or key-policy details into usecase.
