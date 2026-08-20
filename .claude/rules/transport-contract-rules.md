---
description: Transport Contract Rules
---

# Transport Contract Rules

`contracts/http/openapi.yaml` is the single source for API purpose and field meaning. Every endpoint needs `summary` + `description`; every input/response field needs `description`. After changes: `make openapi-generate`, then adapt handlers.

Versioned protobuf files under `contracts/grpc/` are the source of truth for gRPC contracts. Generated Go types stay under `internal/delivery/grpc/gen/` and must not cross into usecase or repository contracts. Preserve existing field numbers; after changes run `make proto-lint proto-generate`, then adapt gRPC services.

