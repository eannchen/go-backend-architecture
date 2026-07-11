# internal/infra/external/objectstorage/r2

## Pattern used

- Adapts the S3-compatible Cloudflare R2 API to the object-storage repository contract.
- Keeps R2 endpoint and SDK configuration inside infra.

## How to extend

- Build `Config` from feature-specific configuration in the app composition package.
- Add provider behavior here without exposing AWS SDK types beyond this package.
