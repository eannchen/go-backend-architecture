# internal/delivery/http/integration

## Pattern used

- Feature tests run the HTTP server in-process with disposable PostgreSQL and Redis containers.
- Fixture files own dependency wiring, request helpers, and cleanup; flow files read as client-visible scenarios.
- External providers stay fake so tests remain deterministic and do not send network requests outside their containers.

## How to extend

- Keep multi-step workflows explicit; use tables only when every case follows the same arrange, act, and assert sequence.
- Assert client-visible behavior here and keep SQL, key-format, serialization, and TTL assertions in adapter tests.
- Register cleanup for every test and add new integration packages to `INTEGRATION_PACKAGES` in the Makefile.
