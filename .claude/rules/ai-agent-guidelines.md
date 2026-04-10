---
description: AI Agent Guidelines
---

# AI Agent Guidelines

1. Read the `README.md` in any package directory before modifying it — it contains the pattern used and how to extend.
2. Search repo for existing patterns first.
3. Follow architecture and dependency boundaries.
4. Prefer modifying existing structures over new abstractions.
5. Use same constructor and wiring patterns.
6. Match commenting style; skip unnecessary comments.
7. For HTTP changes: update `docs/openapi.yaml` first, run `make openapi-generate`, then adapt handlers.
8. Use binding tags on DTOs (`trim:"false"`, `case:"lower"`, `case:"upper"`); no manual trim/case.
9. Follow **File and directory naming** conventions above.
10. Avoid redundant passes over the same data unless clarity or separation is worth it.
