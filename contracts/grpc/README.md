# contracts/grpc

## Pattern used

- Versioned protobuf packages are the source of truth; generated Go transport types are committed under `internal/delivery/grpc/gen`.
- Buf's `STANDARD` lint policy keeps contracts consistent, while its `FILE` breaking policy protects generated-code and wire compatibility.
- CI compares changes with the exact pull-request base or pre-push commit and verifies that generation produces no uncommitted output.

## How to extend

- Preserve existing field numbers. Reserve the numbers and names of removed fields so they cannot be reused accidentally.
- Add compatible methods and fields to the current version; create a new versioned package for an intentional breaking redesign instead of weakening the policy.
- Run `make proto-check` before committing. It uses local `main` by default; override `PROTO_BREAKING_BASE_REF` and `PROTO_BREAKING_AGAINST` when another baseline is required.
