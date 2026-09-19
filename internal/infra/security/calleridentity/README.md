# Certificate caller identity

This package implements certificate-to-identity policies for verified client certificates.

## Responsibilities and boundaries

- The extractor requires exactly one URI Subject Alternative Name (SAN) and rejects missing or ambiguous identities.
- Legacy Common Name values are not accepted as identity.
- gRPC peer inspection and business authorization remain outside this package.

## Extending

- Add an extractor only for an explicit identity convention supported by the certificate authority.
- Return the shared `internal/security/calleridentity.Identity` type.
- Reject ambiguous certificates rather than selecting one identity silently.
