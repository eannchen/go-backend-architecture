# internal/infra

Infrastructure adapters and external integrations.

## Pattern used

- Adapter pattern: implement inner-layer contracts.
- Keep package boundaries explicit (`db`, `logger`, `observability`, etc.).
- Hide vendor-specific code behind project contracts.

## How to extend

- Add a new subpackage per infrastructure concern.
- Keep constructors explicit and injectable.
- Expose only contract-friendly types to outer packages.
