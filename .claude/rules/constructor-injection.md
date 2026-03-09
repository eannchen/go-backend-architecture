---
description: Constructor Injection
---

# Constructor Injection

Use constructor injection only. No service locator or global containers. Dependencies must be explicit.

Examples:

- Handler: `NewHandler(log, tracer, usecase)`
- Usecase: `New(log, tracer, repo)`

