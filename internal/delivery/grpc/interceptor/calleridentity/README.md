# gRPC caller identity

This interceptor derives an authenticated machine identity from a verified client certificate.

## Responsibilities and boundaries

- Reads only certificates accepted by the TLS verified chain; a presented but unverified certificate never establishes identity.
- Delegates certificate interpretation to the injected `CertificateExtractor`.
- Adds identity to unary and streaming contexts before observability and service execution.
- Leaves calls anonymous when client certificates are optional and none is verified.

## Extending

- Inject an extractor matching the certificate authority's documented identity convention.
- Keep authorization decisions in usecases or a dedicated authorization interceptor.
- Do not infer identity from unverified peer data.
