---
applyTo: "**.go"
---

# General Instructions for Go

## Comments and Documentation
- Document all exported items - every exported function, type, constant, and variable must have a doc comment.
- Use Go doc links to reference other identifiers, e.g. `[Name1]` or `[Name1.Name2]`.
- Create package documentation in dedicated `doc.go` files for comprehensive package overviews.

## Error Handling
- Use global error variables for common errors.
- Wrap all errors with `fmt.Errorf` to add context.
- Keep error messages short and to the point, since they may be wrapped many times.
- Error messages should not contain the words `failed` or `error`, since this will be redundant after wrapping.

Example patterns to follow:

```go
var (
    ErrNotFound      = errors.New("resource not found")
    ErrInvalidInput  = errors.New("invalid input")
    ErrUnauthorized  = errors.New("unauthorized access")
)

// Wrap errors with context.
if err != nil {
    return fmt.Errorf("validate user: %w", err)
}

// Chain context and specific errors.
if err != nil {
    return fmt.Errorf("%w: %w", ErrNotFound, err)
}
```

## Refactoring and API Evolution
- **Embrace breaking changes** - we are the only consumers of this codebase, so API compatibility is not a concern.
- **Prefer good design over backward compatibility** - choose the better solution even if it requires updating existing code.

## Testing
- **ALWAYS** read and follow the test instructions in `test.instructions.md` when working with test files (`*_test.go`).

## Tools
- **Formatting**: Use `devbox run -- task go-format` to format code.
