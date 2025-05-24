---
applyTo: "**/*_test.go"
---

# Testing Instructions for Go

## Test Guidelines

- Use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` for assertions.
- Check the type of errors using `require.ErrorIs`, which can be used to check against our global error variables.
- Use Go 1.24, so you can use features from that version. Notably, you do not need to use `tc := tc` since Go 1.24 does not require it.

## Test Structure

Use table-driven tests where possible. For example:

```go
tcs := map[string]struct {
	input string
	want  string
}{
	"test case one": {
		input: "foo",
		want: "bar",
	},
	"test case two": {
		input: "baz",
		want: "qux",
	},
}
for name, tc := range tcs {
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		got := someFunction(tc.input)
		assert.Equal(t, tc.want, got)
	})
}
```
