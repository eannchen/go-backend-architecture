---
description: AI Agent Guidelines
---

# AI Agent Guidelines

1. Search repo for existing patterns first.
2. Follow architecture and dependency boundaries.
3. Prefer modifying existing structures over new abstractions.
4. Use same constructor and wiring patterns.
5. Match commenting style; skip unnecessary comments.
6. For HTTP changes: update `docs/openapi.yaml` first, run `make openapi-generate`, then adapt handlers.
7. Use binding tags on DTOs (`trim:"false"`, `case:"lower"`, `case:"upper"`); no manual trim/case.
8. Follow **File and directory naming** conventions above.
9. Avoid redundant passes over the same data unless clarity or separation is worth it.
