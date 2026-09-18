# HTTP request binding

This package normalizes bound request fields before validation and handler execution.

## Responsibilities and boundaries

- `NormalizeBinder` delegates decoding to an `echo.Binder`, then applies string normalization expressed by struct tags.
- Supported normalization includes trimming, case conversion, pointers, named strings, nested structs, and slices.
- The binder changes transport input only; business normalization remains outside this package.

## Extending

- Express field behavior with `trim` and `case` tags, including OpenAPI-generated extra tags.
- Add a normalization rule with focused reflection and binding tests.
- Inject a custom `echo.Binder` through app wiring when decoding behavior must change.
