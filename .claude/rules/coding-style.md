---
description: Coding Style
---

# Coding Style

Idiomatic Go. Naming: exported `PascalCase`, unexported `camelCase`, constructors `NewX(...)`. Interfaces describe behavior (`Usecase`, `RouteRegistrar`, `TxManager`); avoid `I*` prefixes. Keep functions small; use named returns only when they improve clarity.

