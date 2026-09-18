# HTTP application composition

This package assembles and runs the public HTTP API process.

## Responsibilities and boundaries

- Builds infrastructure, repository adapters, usecases, handlers, middleware, and the HTTP server in dependency order.
- Returns startup failures immediately and closes resources that were already initialized.
- Contains wiring only; request mapping belongs in delivery and business behavior belongs in usecases or domain code.

## Extending

- Add constructors to the matching `httpapi_*_wiring.go` file.
- Put setup shared by multiple executable types in `internal/app/runtime`.
- Preserve explicit construction and reverse-order cleanup when adding dependencies.
