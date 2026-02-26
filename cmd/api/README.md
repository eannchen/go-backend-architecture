# cmd/api

Process entrypoint package.

- Owns boot lifecycle only (construct app, start server, handle shutdown signals).
- Must not contain business rules.
