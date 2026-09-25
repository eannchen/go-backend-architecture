---
name: tdd-change
description: Run one phase or a complete red-green-refactor cycle for a small change in this Go backend. Use when the user asks for TDD, Red, Green, Refactor, or a test-first workflow; do not use for ordinary implementation requests.
---

# TDD Change

Follow the architecture and testing rules in `AGENTS.md`. This skill defines the TDD steps and where to stop.

## Choose the mode

Use the mode named by the user. Use conversation context when the user says something like “do the Green” after completing Red.

- **Red only:** write tests for one behavior, confirm they fail for the expected reason, and stop.
- **Green only:** start with the failing tests, make them pass, and stop.
- **Refactor only:** start with passing tests, improve the code if useful, confirm the tests still pass, and stop.
- **Complete cycle:** perform Red, Green, and Refactor in order.

When no mode is given, use the complete cycle. Stop after one phase when the user requests that phase or has said the phases will be done separately.

Keep the code responsible for the behavior real. Replace a dependency when the test needs to control its result or avoid exercising another layer or external system. Add the test comment required by `AGENTS.md`.

One behavior may need several test functions or table cases. They can share a cycle when one implementation can make all of them pass. Use separate cycles for independent behaviors.

## Red only

1. State the behavior and why this is the right place to test it.
2. Add or change the tests and their setup. Follow `AGENTS.md` when a test double is needed. Do not implement the requested production behavior.
3. Run the narrowest test command against unchanged production code.
4. Confirm the command fails because the requested behavior is missing. If the target function or method does not exist, its compile error is Red; create it during Green.
5. Test or setup mistakes, unavailable services, and unrelated failures are not Red. Fix them or report that Red could not be verified.
6. If the tests pass, check whether the behavior already exists. Otherwise, change the tests so the current and requested behaviors produce different results.
7. Stop and report the test locations, command, and expected failure.

## Green only

Green requires the failing tests from Red.

1. Run those tests before editing production code and confirm they still fail for the expected reason.
2. If those failing tests do not exist, stop and explain that Green cannot begin. Do not write the missing Red tests unless the user allowed it.
3. Make the smallest clean production change that makes the tests pass. This includes creating a missing target function or method. Follow existing repository patterns.
4. Do not weaken assertions to make tests pass. Change a test only when it is wrong, and explain why.
5. Run the focused tests and all tests in the affected packages.
6. Stop when they pass. Leave optional cleanup for Refactor.

## Refactor only

1. Run the relevant focused and package tests and confirm they pass before editing.
2. Look for a clear improvement in names, duplication, responsibilities, control flow, test setup, or assertions. Make no change when the code is already clear.
3. Improve production or test code without changing behavior. Do not move important test setup or assertions into helpers that make the test harder to understand.
4. Run the focused tests and checks appropriate for the changed code.
5. Report the improvement or explain why no refactor was needed.

## Complete cycle

Perform Red, Green, and Refactor in order for one behavior. Record:

1. Red test command and expected failure.
2. Green production change and passing tests.
3. Refactor choice and final checks.

Finish the cycle for one behavior before starting another.

## Generated contracts and wiring

For OpenAPI or Protobuf changes, update the source contract and generate code first. Then begin Red with the handler or service mapping test. Never edit generated files directly.

When wiring only changes constructor calls, rely on the compiler and run a focused build. Add a test when wiring changes runtime behavior, such as routes, middleware order, TLS, or identity.

## Handoff

Keep reports concise and clear. Include the behavior, command and result, and the next phase when one remains. Do not claim a later phase was completed.
