# internal/delivery/http/openapi/gen

## Pattern used

- `contracts/http/openapi.yaml` is the source contract.
- `oapi-codegen` generates transport models into this package.

## How to extend

- Update `contracts/http/openapi.yaml` first, then `make openapi-generate`.
- Do not edit generated files manually.
