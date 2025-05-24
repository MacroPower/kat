---
applyTo: "**.go"
---

# General Instructions for Go

## Comments

- Always use complete sentences. Make sure to end sentences with a period.
- Use doc links in the form of `[Name1]` or `[Name1.Name2]` to refer to exported identifiers in the current package, or `[pkg]`, `[pkg.Name1]`, or `[pkg.Name1.Name2]` to refer to identifiers in other packages. Use a leading star to refer to pointer types, e.g. `[*bytes.Buffer]`.

## Error Handling

- Use global error variables for common errors.
- Wrap all errors with `fmt.Errorf` to add context.
- Keep error messages short and to the point, since they may be wrapped many times.
- Error messages should not contain the words `failed` or `error`, since this will be redundant after wrapping.

Example:

```go
var ErrNotFound = errors.New("resource not found")

// ...

if err != nil {
	return fmt.Errorf("%w: %w", ErrNotFound, err)
}
```
