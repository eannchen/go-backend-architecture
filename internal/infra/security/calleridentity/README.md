# internal/infra/security/calleridentity

## Pattern used

- Implements certificate-to-identity policies defined by `internal/security/calleridentity`.
- The default implementation requires one URI subject alternative name and does not infer identity from the legacy Common Name.

## How to extend

- Add another extractor when the certificate authority uses a different explicit identity convention.
- Keep gRPC peer inspection in delivery and authorization decisions outside this package.
