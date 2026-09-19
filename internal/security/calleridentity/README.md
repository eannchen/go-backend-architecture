# Caller identity contract

This package defines the transport-neutral identity of an authenticated machine caller.

## Responsibilities and boundaries

- Owns the identity value, context helpers, and certificate extractor contract.
- Delivery extracts transport credentials; infrastructure implements certificate interpretation; usecases receive identity explicitly when authorization needs it.
- Identity fields have the same meaning regardless of the authentication mechanism.

## Extending

- Add a field only when its meaning is consistent across identity sources; make it optional when a source cannot provide it.
- Keep certificate, token, and gateway parsing in their adapters.
- Pass identity into business methods instead of reading context when authorization is part of the business rule.
