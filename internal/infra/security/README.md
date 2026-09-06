# internal/infra/security

## Pattern used

- Security infrastructure loads and validates process-owned cryptographic material without exposing file I/O to delivery packages.

## How to extend

- Keep protocol-neutral TLS construction reusable and wrap it with protocol credentials in the app composition layer.
