# internal/repository/external/objectstorage

## Pattern used

- Defines the feature-neutral port for storing objects and signing read URLs.
- Exposes only object keys, content bytes, and metadata; provider SDK types stay in infra.

## How to extend

- Add only operations a usecase needs.
- Implement the contract in an `internal/infra/external/objectstorage/<provider>` package.
