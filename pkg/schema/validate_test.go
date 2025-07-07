// Package schema_test contains tests for the schema package's public interface.
// Tests are in a separate package to ensure we only test exported functionality.
package schema_test

import (
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/schema"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err  schema.ValidationError
		want string
	}{
		"with path": {
			err: schema.ValidationError{
				Path:   mustBuildPath(t, "field", "subfield"),
				Detail: "value is required",
			},
			want: "error at $.field.subfield: value is required",
		},
		"without path": {
			err: schema.ValidationError{
				Detail: "value is required",
			},
			want: "validation error: value is required",
		},
		"empty detail": {
			err: schema.ValidationError{
				Path:   mustBuildPath(t, "field"),
				Detail: "",
			},
			want: "error at $.field: ",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.err.Error()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewValidator(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		errMsg     string
		schemaData []byte
		wantErr    bool
	}{
		"valid schema": {
			schemaData: []byte(`{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"}
				},
				"required": ["name"]
			}`),
			wantErr: false,
		},
		"invalid json": {
			schemaData: []byte(`{"invalid": json}`),
			wantErr:    true,
			errMsg:     "unmarshal schema",
		},
		"invalid schema": {
			schemaData: []byte(`{"type": "invalid_type"}`),
			wantErr:    true,
			errMsg:     "compile schema",
		},
		"empty schema": {
			schemaData: []byte(`{}`),
			wantErr:    false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			validator, err := schema.NewValidator(tc.schemaData)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, validator)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, validator)
			}
		})
	}
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

	schemaData := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"},
			"items": {
				"type": "array",
				"items": {"type": "string"}
			},
			"nested": {
				"type": "object",
				"properties": {
					"value": {"type": "string"}
				},
				"required": ["value"]
			},
			"users": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "number"},
						"email": {"type": "string"},
						"profile": {
							"type": "object",
							"properties": {
								"firstName": {"type": "string"},
								"lastName": {"type": "string"},
								"preferences": {
									"type": "array",
									"items": {"type": "string"}
								}
							},
							"required": ["firstName", "lastName"]
						}
					},
					"required": ["id", "email", "profile"]
				}
			},
			"matrix": {
				"type": "array",
				"items": {
					"type": "array",
					"items": {"type": "number"}
				}
			}
		},
		"required": ["name"]
	}`)

	validator, err := schema.NewValidator(schemaData)
	require.NoError(t, err)

	tcs := map[string]struct {
		data         any
		expectedPath string
		wantErr      bool
	}{
		"valid data": {
			data: map[string]any{
				"name": "John",
				"age":  30,
			},
			wantErr: false,
		},
		"missing required field": {
			data: map[string]any{
				"age": 30,
			},
			wantErr:      true,
			expectedPath: "$",
		},
		"wrong type for name": {
			data: map[string]any{
				"name": 123,
				"age":  30,
			},
			wantErr:      true,
			expectedPath: "$.name",
		},
		"wrong type for age": {
			data: map[string]any{
				"name": "John",
				"age":  "thirty",
			},
			wantErr:      true,
			expectedPath: "$.age",
		},
		"invalid array item": {
			data: map[string]any{
				"name":  "John",
				"items": []any{"valid", 123, "also valid"},
			},
			wantErr:      true,
			expectedPath: "$.items[1]",
		},
		"nested object validation error": {
			data: map[string]any{
				"name": "John",
				"nested": map[string]any{
					"notValue": "something",
				},
			},
			wantErr:      true,
			expectedPath: "$.nested",
		},
		"valid array of objects": {
			data: map[string]any{
				"name": "John",
				"users": []any{
					map[string]any{
						"id":    1,
						"email": "john@example.com",
						"profile": map[string]any{
							"firstName": "John",
							"lastName":  "Doe",
						},
					},
					map[string]any{
						"id":    2,
						"email": "jane@example.com",
						"profile": map[string]any{
							"firstName":   "Jane",
							"lastName":    "Smith",
							"preferences": []any{"dark_mode", "notifications"},
						},
					},
				},
			},
			wantErr: false,
		},
		"invalid object in array": {
			data: map[string]any{
				"name": "John",
				"users": []any{
					map[string]any{
						"id":    1,
						"email": "john@example.com",
						"profile": map[string]any{
							"firstName": "John",
							"lastName":  "Doe",
						},
					},
					map[string]any{
						"id":    "invalid", // should be number
						"email": "jane@example.com",
						"profile": map[string]any{
							"firstName": "Jane",
							"lastName":  "Smith",
						},
					},
				},
			},
			wantErr:      true,
			expectedPath: "$.users[1].id",
		},
		"missing required field in nested object within array": {
			data: map[string]any{
				"name": "John",
				"users": []any{
					map[string]any{
						"id":    1,
						"email": "john@example.com",
						"profile": map[string]any{
							"firstName": "John",
							// missing lastName
						},
					},
				},
			},
			wantErr:      true,
			expectedPath: "$.users[0].profile",
		},
		"invalid preference in deeply nested array": {
			data: map[string]any{
				"name": "John",
				"users": []any{
					map[string]any{
						"id":    1,
						"email": "john@example.com",
						"profile": map[string]any{
							"firstName": "John",
							"lastName":  "Doe",
							"preferences": []any{
								"dark_mode",
								123, // should be string
								"notifications",
							},
						},
					},
				},
			},
			wantErr:      true,
			expectedPath: "$.users[0].profile.preferences[1]",
		},
		"valid matrix (2D array)": {
			data: map[string]any{
				"name": "John",
				"matrix": []any{
					[]any{1, 2, 3},
					[]any{4, 5, 6},
					[]any{7, 8, 9},
				},
			},
			wantErr: false,
		},
		"invalid element in 2D array": {
			data: map[string]any{
				"name": "John",
				"matrix": []any{
					[]any{1, 2, 3},
					[]any{4, "invalid", 6}, // should be number
					[]any{7, 8, 9},
				},
			},
			wantErr:      true,
			expectedPath: "$.matrix[1][1]",
		},
		"missing email in second user": {
			data: map[string]any{
				"name": "John",
				"users": []any{
					map[string]any{
						"id":    1,
						"email": "john@example.com",
						"profile": map[string]any{
							"firstName": "John",
							"lastName":  "Doe",
						},
					},
					map[string]any{
						"id": 2,
						// missing email
						"profile": map[string]any{
							"firstName": "Jane",
							"lastName":  "Smith",
						},
					},
				},
			},
			wantErr:      true,
			expectedPath: "$.users[1]",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validator.Validate(tc.data)

			if tc.wantErr {
				require.Error(t, err)
				var validationErr *schema.ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.NotNil(t, validationErr.Path)
				assert.Equal(t, tc.expectedPath, validationErr.Path.String())
				assert.NotEmpty(t, validationErr.Detail)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test that [schema.ValidationError.Path] works correctly with [yaml.Path.AnnotateSource].
func TestValidator_Validate_WithAnnotateSource(t *testing.T) {
	t.Parallel()

	schemaData := []byte(`{
		"type": "object",
		"properties": {
			"users": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"age": {"type": "number"}
					},
					"required": ["name", "age"]
				}
			},
			"config": {
				"type": "object",
				"properties": {
					"enabled": {"type": "boolean"},
					"settings": {
						"type": "array",
						"items": {"type": "string"}
					}
				},
				"required": ["enabled"]
			}
		},
		"required": ["users"]
	}`)

	validator, err := schema.NewValidator(schemaData)
	require.NoError(t, err)

	tcs := map[string]struct {
		yamlSource       string
		expectedErrorMsg string
		expectedLine     int // Expected line number where error should be annotated
	}{
		"missing required field at root": {
			yamlSource: `config:
  enabled: true
  settings:
    - "option1"
    - "option2"`,
			expectedErrorMsg: "error at $:",
			expectedLine:     1, // Error at root, should point to first line
		},
		"wrong type in array element": {
			yamlSource: `users:
  - name: "John"
    age: 30
  - name: "Jane"
    age: "invalid"
config:
  enabled: true`,
			expectedErrorMsg: "error at $.users[1].age:",
			expectedLine:     5, // Line with 'age: "invalid"'
		},
		"missing required field in array object": {
			yamlSource: `users:
  - name: "John"
    age: 30
  - name: "Jane"
    # missing age field
config:
  enabled: true`,
			expectedErrorMsg: "error at $.users[1]:",
			expectedLine:     4, // Line with 'name: "Jane"' (start of problematic object)
		},
		"wrong type in nested array": {
			yamlSource: `users:
  - name: "John"
    age: 30
config:
  enabled: true
  settings:
    - "option1"
    - 123
    - "option3"`,
			expectedErrorMsg: "error at $.config.settings[1]:",
			expectedLine:     8, // Line with '- 123'
		},
		"missing required field in nested object": {
			yamlSource: `users:
  - name: "John"
    age: 30
config:
  settings:
    - "option1"`,
			expectedErrorMsg: "error at $.config:",
			expectedLine:     5, // Line with 'settings:' (first property of problematic object)
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Parse the YAML source
			var data any
			err := yaml.Unmarshal([]byte(tc.yamlSource), &data)
			require.NoError(t, err, "YAML should be valid")

			// Validate the data
			err = validator.Validate(data)
			require.Error(t, err, "Validation should fail")

			// Check that we get a ValidationError
			var validationErr *schema.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.NotNil(t, validationErr.Path, "ValidationError should have a path")

			// Check that the error message contains the expected path
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)

			// Test AnnotateSource functionality
			annotated, err := validationErr.Path.AnnotateSource([]byte(tc.yamlSource), true)
			require.NoError(t, err, "AnnotateSource should not fail")

			// The annotated source should contain error markers
			annotatedStr := string(annotated)
			assert.NotEqual(t, tc.yamlSource, annotatedStr, "Annotated source should be different from original")

			// Debug output
			t.Logf("Expected line: %d", tc.expectedLine)
			t.Logf("Annotated output:\n%s", annotatedStr)

			// Split into lines to check annotation
			lines := splitLines(annotatedStr)

			// The annotated source might only show a subset of lines around the error.
			// Look for the expected line number in the annotated output.
			expectedLineStr := fmt.Sprintf("%d |", tc.expectedLine)
			foundExpectedLine := false
			var errorLine string

			for _, line := range lines {
				if containsString(line, expectedLineStr) {
					foundExpectedLine = true
					errorLine = line

					break
				}
			}

			assert.True(t, foundExpectedLine, "Should find line %d in annotated output", tc.expectedLine)

			// Check that the found line or surrounding lines contain error markers
			if foundExpectedLine {
				hasAnnotation := containsErrorMarkers(errorLine)

				// Also check the next line for caret markers
				if !hasAnnotation {
					for i, line := range lines {
						if containsString(line, expectedLineStr) && i+1 < len(lines) {
							hasAnnotation = containsErrorMarkers(lines[i+1])

							break
						}
					}
				}

				assert.True(t, hasAnnotation, "Line %d should have error annotation markers", tc.expectedLine)
			}
		})
	}
}

// splitLines splits a string into lines, preserving empty lines.
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}

	lines := []string{}
	start := 0

	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	// Add the last line if it doesn't end with newline
	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}

// containsErrorMarkers checks if a line contains YAML annotation error markers.
// The go-yaml library uses ">" to mark error lines and "^" for caret indicators.
func containsErrorMarkers(line string) bool {
	// Check for line marker (starts with ">")
	if line != "" && line[0] == '>' {
		return true
	}

	// Check for caret indicator line (contains "^")
	return containsString(line, "^")
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && indexString(s, substr) >= 0
}

// indexString returns the index of the first occurrence of substr in s, or -1.
func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}

// mustBuildPath is a helper function to build YAML paths for testing.
// It panics on error, making it suitable only for test cases where the path is known to be valid.
func mustBuildPath(t *testing.T, parts ...string) *yaml.Path {
	t.Helper()

	pb := yaml.PathBuilder{}
	current := pb.Root()

	for _, part := range parts {
		current = current.Child(part)
	}

	path := current.Build()

	return path
}
