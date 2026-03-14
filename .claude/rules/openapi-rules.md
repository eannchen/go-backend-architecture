---
description: OpenAPI Rules
---

# OpenAPI Rules

`docs/openapi.yaml` is the single source for API purpose and field meaning. Every endpoint needs `summary` + `description`; every input/response field needs `description`. After changes: `make openapi-generate`, then regenerate `docs/insomnia.json`.

