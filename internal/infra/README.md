# Infrastructure adapters

This directory contains concrete implementations of domain, usecase, and shared technical contracts.

## Responsibilities and boundaries

- Database, cache, key-value, messaging, and provider SDK details remain inside infrastructure packages.
- Adapters map vendor types and errors to types owned by the consuming domain or usecase package. Importing those contracts does not permit calling workflow implementations.
- Constructors expose dependencies explicitly; app composition chooses and owns concrete instances.

## Extending

- Implement the contract defined by the consumer. Low-level clients with no application consumer can expose concrete types without a speculative interface.
- Keep provider configuration and lifecycle ownership explicit.
- Do not return driver, SDK, or framework types through domain or usecase contracts.
