# Infrastructure adapters

This directory contains concrete implementations of repository and shared technical contracts.

## Responsibilities and boundaries

- Database, cache, key-value, messaging, provider, logging, observability, and security SDK details remain inside infrastructure packages.
- Adapters map vendor types and errors to contract-friendly types before returning.
- Constructors expose dependencies explicitly; app composition chooses and owns concrete instances.

## Extending

- Use an existing contract when one represents the required behavior. Add a contract for a new outbound capability; low-level clients do not need a repository contract of their own.
- Keep provider configuration and lifecycle ownership explicit.
- Do not return driver, SDK, or framework types through repository contracts.
