# cmd/api

Process entrypoint package.

## Pattern used

- Owns process boot lifecycle only (construct app, start server, handle shutdown signals).
- Keeps orchestration thin and delegates wiring to `internal/app`.

## How to extend

- Add process-level concerns only (signal handling, startup/shutdown flow).
- Do not place business/usecase logic in this package.
