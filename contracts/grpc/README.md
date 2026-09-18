# gRPC contracts

This directory contains the versioned Protocol Buffer contracts used by the gRPC adapter.

## Responsibilities and boundaries

- Protobuf files are the source of truth; generated Go files belong in `internal/delivery/grpc/gen`.
- `buf.yaml` selects Buf's `STANDARD` lint category, its recommended rules for Protobuf naming, packages, and structure.
- It also selects `FILE`, Buf's strictest breaking-change category. The category checks compatibility per `.proto` file,
  including Protobuf's binary wire format, JSON format, and generated source, so moving a declaration between files is breaking.
- When removing a field, reserve both its number and name so neither can be reused accidentally.

## Extending

- Add compatible methods or fields to the current version; create a new version for an intentional breaking redesign.
- Run `make proto-check` after changes. Set `PROTO_BREAKING_BASE_REF` to compare with a local Git ref other than `main`.
