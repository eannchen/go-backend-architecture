---
description: AI Agent Guidelines
---

# AI Agent Guidelines

When generating code:

1. Search the repo for existing patterns before creating new ones.
2. Follow the existing architecture and dependency boundaries.
3. Prefer modifying existing structures over new abstractions.
4. Use the same constructor and wiring patterns as the codebase.
5. Match commenting style of nearby code; add comments for complex logic, skip unnecessary ones.
6. For HTTP contract changes, update `docs/openapi.yaml` first, run `make openapi-generate`, and then adapt delivery handlers.

The architecture should stay predictable for humans, AI agents, and future maintainers.
