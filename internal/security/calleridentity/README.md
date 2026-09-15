# internal/security/calleridentity

## Pattern used

- Defines the transport-neutral identity of an authenticated machine caller.
- Owns the certificate extractor contract implemented by security infrastructure.
- Stores identity in `context.Context` so delivery, observability, and explicitly identity-aware application code can share it without importing gRPC or TLS.

## How to extend

- Keep certificate, token, and gateway parsing in their transport adapters.
- Add identity fields only when they have the same meaning across authentication mechanisms.
- Pass identity explicitly to business methods when authorization is part of the business rule.
