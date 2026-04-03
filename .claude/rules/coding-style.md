---
description: Coding Style
---

# Coding Style

Idiomatic Go. Exported `PascalCase`, unexported `camelCase`, constructors `NewX(...)`. Interfaces describe behavior; avoid `I*` prefixes. Small functions; named returns only when they improve clarity.

**Iteration:** Prefer **one pass** over the same collection when it stays clear (merge derivations, batch SQL, pre-size from known `len`).

