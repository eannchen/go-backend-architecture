# gRPC caller identity

This interceptor derives an authenticated machine identity from a verified client certificate.

## Responsibilities and boundaries

- Reads only certificates accepted by the TLS verified chain; a presented but unverified certificate never establishes identity.
- Requires exactly one nonempty URI SAN on the verified certificate to identify the caller.
- Stores identity through `grpccontext`, also used by logging and tracing.
- Adds identity to unary and streaming contexts before observability and service execution.
- Leaves calls anonymous when client certificates are optional and none is verified.

## Extending

- Adapt `extractCallerIdentity` if the certificate authority uses a different identity convention.
- Keep business authorization decisions in usecases. Pass required caller information explicitly into the workflow.
- Do not infer identity from unverified peer data.
