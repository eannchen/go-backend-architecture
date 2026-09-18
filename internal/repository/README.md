# Repository contracts

This directory defines outbound capabilities required by usecases.

## Responsibilities and boundaries

- Subdirectories group contracts by capability area: database, cache, key-value storage, and external providers.
- Interfaces describe application behavior rather than tables, SDKs, or generic create/read/update/delete operations.
- Contracts may return domain entities or result types shaped for a specific operation, but never driver or vendor types.

## Extending

- Add the smallest capability required by a usecase in the matching area.
- Group methods by business purpose rather than by schema shape.
- Implement the contract under the corresponding `internal/infra` package and wire it in `internal/app`.
