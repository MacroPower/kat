---
applyTo: "**/*_test.go"
---

# Testing Instructions for Go

## Test Guidelines
- Use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` for assertions.
- Check the type of errors using `require.ErrorIs`, which can be used to check against our global error variables.
- Use `require.ErrorAs` to check if an error can be converted to a specific type.
- Use Go 1.24 features. Notably, you do not need to use `tc := tc` since Go 1.24 does not require it.
- Use `t.Parallel()` in all tests to run them concurrently.
- Use `t.Helper()` to mark helper functions that are only used within tests.
- Use the standardized names `input`, `want`, `got`, and `err` for test case fields to maintain consistency across the codebase.
- Always create a test package (e.g. `package foo_test`) and focus on testing the public API.
- Test both success and error scenarios for each function.
- Use descriptive test case names that explain the scenario being tested.

## Test Assumptions
- Never assume that the code being tested is correct!
- If a result does not seem logical, consider that it might be a bug in the code.
- If after careful consideration you arrive at the conclusion that there is a bug in the code being tested, **propose a fix and ask how to proceed**.

## Test Structure
Always use table-driven tests whenever testing more than one set of inputs/outputs.

This is the **REQUIRED** format:
```go
tcs := map[string]struct {
	input string
	want  string
	err   error
}{
	"successful case with valid input": {
		input: "foo",
		want:  "bar",
		err:   nil,
	},
	"error case with invalid input": {
		input: "baz",
		want:  "",
		err:   ErrInvalidInput,
	},
	"edge case with empty input": {
		input: "",
		want:  "",
		err:   ErrEmptyInput,
	},
}

for name, tc := range tcs {
	t.Run(name, func(t *testing.T) {
		t.Parallel()

		got, err := someFunction(tc.input)

		if tc.err != nil {
			require.Error(t, err)
			require.ErrorIs(t, err, tc.err)
			return
		}

		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	})
}
```

## Testing Checklist
Before completing any test-related task:
- Have I used the table-driven test structure?
- Have I used the required field names (`input`, `want`, `got`, `err`)?
- Have I included `t.Parallel()` in all test functions?
- Have I tested both success and error scenarios?
- Have I used `require.ErrorIs` for error type checking?
- Have I marked helper functions with `t.Helper()`?
- Have I combined related tests into table-driven tests?
