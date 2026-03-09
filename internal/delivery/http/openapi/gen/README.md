# internal/delivery/http/openapi/gen

Generated OpenAPI transport models.

## Pattern used

- `docs/openapi.yaml` is the source contract.
- `oapi-codegen` writes generated transport models into this package.
- Handwritten delivery code imports this package when shared response or contract types are useful.

## How to extend

- Update `docs/openapi.yaml` first when the HTTP contract changes.
- Regenerate this package with `make openapi-generate`.
- Do not edit generated files manually.
