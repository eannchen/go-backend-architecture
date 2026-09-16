# Profile selector

## Pattern used

- Applies one destructive, one-time choice between the public HTTP and service gRPC profiles.
- Uses explicit file ownership manifests, validates every path, and refuses a dirty working tree before changing files.

## How to extend

- Give new profile-specific capabilities complete files or directories, then add those paths to the opposite profile's removal manifest.
- Keep shared files free of transport-specific imports so either selected project still builds.
