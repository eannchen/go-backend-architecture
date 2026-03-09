# internal/infra/db

Database implementation(s). Implements repository contracts; contracts live in `internal/repository`.

## Pattern used

- One subpackage per database or vendor; each owns connection lifecycle and repository implementations.
- Static SQL lives in a dedicated generated layer; dynamic query shape uses a shared builder. All SQL stays in infra.
- Repository interfaces are implemented in store packages; no DB types leak to usecase.

## How to extend

- Add a new subpackage per database/vendor; implement repository contracts there.
- Prefer the static SQL layer for fixed query shape; use the builder only when filters or shape are runtime-conditional.
- Keep vendor imports and connection logic inside this tree; expose only contract types upward.
