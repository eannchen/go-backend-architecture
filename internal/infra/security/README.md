# internal/infra/security

## Pattern used

- Security infrastructure loads and validates process-owned cryptographic material without exposing file I/O to delivery packages.
- `calleridentity` maps verified certificates to transport-neutral caller identities.
- `tlsconfig` builds standard-library client and server TLS configurations.

## How to extend

- Keep protocol-neutral TLS construction reusable and wrap it with protocol credentials in the app composition layer.
- Add certificate identity conventions here and inject them into transport adapters from app wiring.
