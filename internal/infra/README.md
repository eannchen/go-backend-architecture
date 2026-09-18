# Infrastructure adapters

This directory contains concrete implementations of repository and shared technical contracts.

## Responsibilities and boundaries

- Database, cache, key-value, provider, logging, observability, and security SDK details remain inside infrastructure packages.
- Adapters map vendor types and errors to contract-friendly types before returning.
- Constructors expose dependencies explicitly; app composition chooses and owns concrete instances.

## Extending

- Implement an existing contract or add the required contract before adding an adapter.
- Keep provider configuration and lifecycle ownership explicit.
- Do not return driver, SDK, or framework types through repository contracts.
