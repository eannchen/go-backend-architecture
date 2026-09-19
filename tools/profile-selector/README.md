# Profile selector

This one-time tool reduces the full source template to either the public HTTP or service gRPC project shape.

## Responsibilities and boundaries

- Uses explicit file ownership manifests and refuses to modify a dirty or already-selected checkout.
- Combines the selected environment fragments, installs the selected Air live-reload and sqlc generation configurations,
  removes the unused profile and selector workflow, and writes `.template-profile`.
- Removes itself after a successful selection so the operation cannot be run twice.
- Does not bootstrap project identity, connect to services, or modify deployed infrastructure.

## Extending

- Give profile-specific capabilities complete files or directories whenever possible.
- Add each profile-owned path to the opposite profile's removal list in `main.go`.
- Keep shared files free of imports from removable capabilities, then validate both profiles in CI.
