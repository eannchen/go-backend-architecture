---
description: OpenAPI Explanation Rules
---

# OpenAPI Explanation Rules

When updating `docs/openapi.yaml`, keep explanations short and consistent:

1. Add `summary` and `description` for every endpoint.
2. Add `description` for user inputs (header, path, query, and request fields).
3. Add `description` for response schemas and key fields used by frontend or review workflows.
4. Explain domain meaning and behavior, not implementation details.
5. Keep each description to 1-2 lines and avoid repeating obvious type information.
6. After OpenAPI changes, run `make openapi-generate`.

