# internal/delivery/grpc/interceptor/calleridentity

## Pattern used

- Reads only certificates from gRPC's verified TLS chains; a merely presented certificate never establishes identity.
- Delegates certificate-to-identity policy to an extractor injected by app wiring.
- Adds the identity to unary and streaming contexts before observability and services run.
- Calls without a client certificate remain anonymous when the server permits optional client certificates.

## How to extend

- Inject infra's URI-SAN extractor or another `CertificateExtractor` matching the certificate authority's explicit identity convention.
- Keep authorization policy out of this interceptor; services or a separate authorization interceptor decide what an authenticated identity may do.
