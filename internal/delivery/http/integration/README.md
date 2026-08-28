# internal/delivery/http/integration

## Pattern used

- Feature-level HTTP tests run the server in-process with real PostgreSQL and Redis adapters.
- External providers stay fake so the suite is deterministic and does not send network requests outside its containers.

## How to extend

- Test a complete client-visible flow rather than connectivity or individual constructor calls.
- Register cleanup for every persisted row and key; keep one disposable dependency container per package.
