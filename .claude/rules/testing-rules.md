---
description: Testing Rules
---

# Testing Rules

Test behavior at the layer that owns it.

- Business logic and utilities default to table-driven tests. Use explicit tests for infra adapters, HTTP transport/routing, and orchestration-heavy flows where setup differs materially per case.
- New business logic in `usecase` should ship with tests. Handler tests stay focused on bind/normalize/validate, cookie/header behavior, and response mapping.
- Prefer real integration tests for SQL, Redis, persistence, serialization, and external API adapters. Manual verification is fine during development, but important behavior should end up covered by automated integration tests.
- Every bug fix adds a regression test at the layer where the bug lived.
- Reusable test doubles live near the contract in test-only helper packages such as `dbtest`, `cachetest`, `kvstoretest`, `oauthtest`, `otptest`, `loggertest`, and `observabilitytest`. Keep one-off doubles local to the test file.
- Do not duplicate the same stub/fake across test files, and do not chase coverage on generated code or trivial pass-through code.

