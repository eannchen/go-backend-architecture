# Domain model

This directory contains business concepts and invariants shared across application workflows.

## Responsibilities and boundaries

- Domain packages own entities, value objects, and rules that are independent of transport and storage.
- Usecases and repository contracts may use domain types; domain packages do not import those outer layers.
- Workflow orchestration, I/O, transport validation, and application error mapping remain outside the domain.

## Extending

- Add a domain type when it carries shared business meaning or protects an invariant.
- Put a rule on the domain type when it belongs to the business concept rather than one workflow.
- Map database and provider models to domain types at infrastructure boundaries.
