# contracts

## Pattern used

- Transport contracts are grouped by protocol and kept separate from domain and usecase models.
- HTTP uses OpenAPI; gRPC uses versioned Protocol Buffer packages.
- Generated server-side Go types stay under `internal/delivery`.

## How to extend

- Change the source contract first, then run its generation and lint targets.
- Version externally visible gRPC packages and preserve existing field numbers.
- Map generated transport types at the delivery boundary; never pass them into usecases or repositories.
