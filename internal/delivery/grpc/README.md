# internal/delivery/grpc

## Pattern used

- Generated protobuf and gRPC types live in `gen/` and remain transport-only.
- Future service adapters will map generated messages to usecase inputs and outputs.
- Source contracts live in `contracts/grpc/` and are organized by versioned protobuf package.

## How to extend

- Update the `.proto` contract first, then run `make proto-lint proto-generate`.
- Add service implementations outside `gen/`; never edit generated files.
- Keep generated protobuf types out of usecase and repository contracts.
