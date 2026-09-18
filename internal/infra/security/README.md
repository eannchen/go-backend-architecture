# Security infrastructure

This directory implements cryptographic material loading and machine-identity extraction.

## Responsibilities and boundaries

- `tlsconfig` builds standard-library TLS configurations from text-based PEM certificate and private-key files.
- `calleridentity` maps verified certificates to the shared caller-identity contract.
- Delivery inspects transport state; app composition injects security implementations and wraps TLS for its protocol.

## Extending

- Keep file I/O, certificate parsing, and identity conventions in infrastructure.
- Add protocol credential wrappers in app composition, not in reusable TLS code.
- Put authorization policy in usecases or a dedicated delivery adapter.
