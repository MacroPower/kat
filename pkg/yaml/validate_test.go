package yaml_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goccyyaml "github.com/goccy/go-yaml"

	"github.com/macropower/kat/pkg/yaml"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		want string
		err  yaml.Error
	}{
		"with path": {
			err: yaml.Error{
				Err:  errors.New("value is required"),
				Path: mustBuildPath(t, "field", "subfield"),
			},
			want: "error at $.field.subfield: value is required",
		},
		"without path": {
			err: yaml.Error{
				Err: errors.New("validation error: value is required"),
			},
			want: "validation error: value is required",
		},
		"empty detail": {
			err: yaml.Error{
				Err:  errors.New(""),
				Path: mustBuildPath(t, "field"),
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

			validator, err := yaml.NewValidator("test", tc.schemaData)

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

	validator, err := yaml.NewValidator("test", schemaData)
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

				var validationErr *yaml.Error
				require.ErrorAs(t, err, &validationErr)
				assert.NotNil(t, validationErr.Path)
				assert.Equal(t, tc.expectedPath, validationErr.Path.String())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
