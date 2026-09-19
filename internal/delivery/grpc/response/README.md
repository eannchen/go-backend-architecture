# gRPC responses

This package maps delivery and application failures to client-safe gRPC statuses.

## Responsibilities and boundaries

- Context cancellation and deadline failures take precedence over application mapping.
- Response errors expose the intended gRPC status while retaining the original cause for tracing and logging.
- Mapping is transport-only; usecases do not import gRPC codes or status types.

## Extending

- Add a transport mapping when the application error package gains a code.
- Keep validation-specific safe messages at the service call site.
- Add protobuf status details only when they are part of a documented client contract.
