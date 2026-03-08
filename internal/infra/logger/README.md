# internal/infra/logger

Project logging contract and implementations.

## Pattern used

- Interface-first logger contract, implementation in `logger/zap`.
- `Fields` hashmap for structured logs.
- Optional sink adapter for exporting logs to observability backend.

## How to extend

- Keep call-site API stable in `logger_contract.go`.
- Add vendor-specific implementation in a subpackage.
- Do not leak vendor types outside logger package.
