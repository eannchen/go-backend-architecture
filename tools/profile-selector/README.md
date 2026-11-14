# Profile selector

This one-time tool reduces the full source template to one of four profiles: public HTTP, service gRPC, Kafka consumer group worker, or Kafka Share Group worker.

## Responsibilities and boundaries

- Uses explicit file ownership manifests and refuses to modify a dirty or already-selected checkout.
- Combines the selected environment fragments, installs the selected Air live-reload and sqlc generation configurations,
  removes unused profiles and the selector workflow, and writes `.template-profile`.
- Removes itself after a successful selection so the operation cannot be run twice.
- Does not bootstrap project identity, connect to services, or modify deployed infrastructure.

## Extending

- Give profile-specific capabilities complete files or directories whenever possible.
- Update the removal lists in `main.go` so each profile keeps its dependencies and removes unused capabilities.
- Keep shared files free of imports from removable capabilities, then validate every profile in CI.
