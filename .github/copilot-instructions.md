# Instructions

## Error Handling

We wrap all errors with `fmt.Errorf` to add context. We use global error variables for common errors. For example:

```go
var ErrNotFound = errors.New("resource not found")

// ...

if err != nil {
	return fmt.Errorf("%w: %w", ErrNotFound, err)
}
```

For this reason, keep error messages short and to the point, since they may be wrapped many times. They should not contain the words `failed` or `error` except in the root context. This is important to reduce redundancy in the final error message. Some examples of good error messages are:

## Testing

We use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` for assertions.

We check the type of errors using `require.ErrorIs`, which can be used to check against our global error variables.

We use table-driven tests where possible. For example:

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

We use Go 1.24, so you can use features from that version. Notably, you do not need to use `tc := tc` since Go 1.24 does not require it.
