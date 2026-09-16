# internal/delivery/http/openapi/gen

## Pattern used

- `contracts/http/openapi.yaml` is the source contract.
- `oapi-codegen` generates request and response transport models into this package.
- Handlers may bind these models, but generated types must not cross into usecases or repositories.

## How to extend

- Update `contracts/http/openapi.yaml` first, then `make openapi-generate`.
- Use OpenAPI constraints for portable rules and `x-oapi-codegen-extra-tags` for Echo/Go-specific tags.
- Do not edit generated files manually.
