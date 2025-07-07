package schema_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/schema"
)

// Test types for schema generation.
type TestStruct struct {
	// Name is the name of the test.
	Name string `json:"name"`
	// Age is the age of the person.
	Age int `json:"age"`
}

type SimpleStruct struct {
	Value string `json:"value"`
}

// ComplexStruct demonstrates various Go type features for schema generation.
type ComplexStruct struct {
	// MapField contains a map of string to int.
	MapField map[string]int `json:"map_field"`
	// PointerField is an optional string field that may be nil.
	PointerField *string `json:"pointer_field,omitempty"`
	// BasicField is a simple string field.
	BasicField string `json:"basic_field"`
	// NestedStruct is a nested structure for testing.
	NestedStruct SimpleStruct `json:"nested_struct"`
	// OptionalField is an optional field that may not be present.
	OptionalField string `json:"optional_field,omitempty"`
	// SliceField contains a slice of strings.
	SliceField []string `json:"slice_field"`
	// EmbeddedStruct is an embedded struct.
	EmbeddedStruct `json:",inline"`
}

// EmbeddedStruct is used for testing inline embedding.
type EmbeddedStruct struct {
	// EmbeddedValue is a field from an embedded struct.
	EmbeddedValue string `json:"embedded_value"`

	// EmbeddedCount is another embedded field.
	EmbeddedCount int `json:"embedded_count"`
}

// NestedComplexStruct tests deeply nested structures.
type NestedComplexStruct struct {
	// Level1 contains the first level of nesting.
	Level1 Level1Struct `json:"level1"`

	// Items contains a slice of complex items.
	Items []ItemStruct `json:"items"`
}

// Level1Struct represents the first level of nesting.
type Level1Struct struct {
	Metadata map[string]any `json:"metadata"`
	Level2   Level2Struct   `json:"level2"`
}

// Level2Struct represents the second level of nesting.
type Level2Struct struct {
	// DeepValue is a deeply nested value.
	DeepValue string `json:"deep_value"`

	// Numbers contains a slice of numbers.
	Numbers []float64 `json:"numbers"`
}

// ItemStruct represents an item in a collection.
type ItemStruct struct {
	Properties map[string]string `json:"properties"`
	ID         string            `json:"id"`
	Tags       []string          `json:"tags"`
}

func TestNewGenerator(t *testing.T) {
	t.Parallel()

	testStruct := TestStruct{}

	gen := schema.NewGenerator(testStruct)
	gen.Tests = true

	assert.NotNil(t, gen)
	assert.NotNil(t, gen.Reflector)
	assert.NotNil(t, gen.LookupCommentFunc)
}

func TestGenerator_Generate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		reflectTarget any
		packagePaths  []string
		wantError     bool
	}{
		"simple struct without package paths": {
			reflectTarget: SimpleStruct{},
			packagePaths:  []string{},
			wantError:     false,
		},
		"test struct without package paths": {
			reflectTarget: TestStruct{},
			packagePaths:  []string{},
			wantError:     false,
		},
		"complex struct without package paths": {
			reflectTarget: ComplexStruct{},
			packagePaths:  []string{},
			wantError:     false,
		},
		"nested complex struct without package paths": {
			reflectTarget: NestedComplexStruct{},
			packagePaths:  []string{},
			wantError:     false,
		},
		"with invalid package paths": {
			reflectTarget: TestStruct{},
			packagePaths:  []string{"invalid/package/path"},
			wantError:     false, // packages.Load returns packages with errors, but doesn't return an error itself
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gen := schema.NewGenerator(tc.reflectTarget, tc.packagePaths...)
			gen.Tests = true
			result, err := gen.Generate()

			if tc.wantError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)

				// Validate that the result is valid JSON.
				var schemaData map[string]any
				err = json.Unmarshal(result, &schemaData)
				require.NoError(t, err)

				// Check basic schema structure - when using references, the schema has $ref and $defs.
				if ref, hasRef := schemaData["$ref"]; hasRef {
					assert.NotNil(t, ref)
					assert.Contains(t, schemaData, "$defs")
				} else {
					// Direct schema structure.
					assert.Contains(t, schemaData, "type")
					assert.Equal(t, "object", schemaData["type"])
					assert.Contains(t, schemaData, "properties")
				}
			}
		})
	}
}

func TestDefaultLookupCommentFunc(t *testing.T) {
	t.Parallel()

	commentMap := map[string]string{
		"schema_test.TestStruct":      "TestStruct is a test structure.",
		"schema_test.TestStruct.Name": "Name field comment.",
		"schema_test.TestStruct.Age":  "Age field comment.",
	}

	lookupFunc := schema.DefaultLookupCommentFunc(commentMap)
	testType := reflect.TypeOf(TestStruct{})

	tcs := map[string]struct {
		fieldName string
		want      string
	}{
		"type comment": {
			fieldName: "",
			want:      "TestStruct is a test structure.\n\nTestStruct: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
		"field comment for Name": {
			fieldName: "Name",
			want:      "Name field comment.\n\nTestStruct.Name: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
		"field comment for Age": {
			fieldName: "Age",
			want:      "Age field comment.\n\nTestStruct.Age: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
		"missing field comment": {
			fieldName: "MissingField",
			want:      "TestStruct.MissingField: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := lookupFunc(testType, tc.fieldName)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultLookupCommentFunc_EmptyCommentMap(t *testing.T) {
	t.Parallel()

	commentMap := map[string]string{}
	lookupFunc := schema.DefaultLookupCommentFunc(commentMap)
	testType := reflect.TypeOf(TestStruct{})

	tcs := map[string]struct {
		fieldName string
		want      string
	}{
		"type without comment": {
			fieldName: "",
			want:      "TestStruct: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
		"field without comment": {
			fieldName: "Name",
			want:      "TestStruct.Name: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#TestStruct",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := lookupFunc(testType, tc.fieldName)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGenerator_SetCustomLookupFunc(t *testing.T) {
	t.Parallel()

	gen := schema.NewGenerator(TestStruct{})
	gen.Tests = true

	// Create a custom lookup function.
	customLookupFunc := func(_ map[string]string) func(t reflect.Type, f string) string {
		return func(_ reflect.Type, _ string) string {
			return "custom comment"
		}
	}

	// Set the custom lookup function.
	gen.LookupCommentFunc = customLookupFunc

	// The function should be set.
	assert.NotNil(t, gen.LookupCommentFunc)

	// Test that the custom function works.
	commentMap := make(map[string]string)
	lookupFunc := gen.LookupCommentFunc(commentMap)
	testType := reflect.TypeOf(TestStruct{})
	result := lookupFunc(testType, "Name")
	assert.Equal(t, "custom comment", result)
}

func TestGenerator_Generate_ComplexStructures(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		reflectTarget any
		checkSchema   func(t *testing.T, schemaData map[string]any)
		packagePaths  []string
	}{
		"complex struct with various field types": {
			reflectTarget: ComplexStruct{},
			packagePaths:  []string{},
			checkSchema: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				// Should have $ref structure for complex types.
				assert.Contains(t, schemaData, "$ref")
				assert.Contains(t, schemaData, "$defs")

				defs, ok := schemaData["$defs"].(map[string]any)
				require.True(t, ok, "Expected $defs to be a map[string]any")
				complexStructDef, ok := defs["ComplexStruct"].(map[string]any)
				require.True(t, ok, "Expected ComplexStruct to be a map[string]any")
				properties, ok := complexStructDef["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// Check basic field.
				basicField, ok := properties["basic_field"].(map[string]any)
				require.True(t, ok, "Expected basic_field to be a map[string]any")
				assert.Equal(t, "string", basicField["type"])

				// Check slice field.
				sliceField, ok := properties["slice_field"].(map[string]any)
				require.True(t, ok, "Expected slice_field to be a map[string]any")
				assert.Equal(t, "array", sliceField["type"])
				items, ok := sliceField["items"].(map[string]any)
				require.True(t, ok, "Expected items to be a map[string]any")
				assert.Equal(t, "string", items["type"])

				// Check map field.
				mapField, ok := properties["map_field"].(map[string]any)
				require.True(t, ok, "Expected map_field to be a map[string]any")
				assert.Equal(t, "object", mapField["type"])
				additionalProps, ok := mapField["additionalProperties"].(map[string]any)
				require.True(t, ok, "Expected additionalProperties to be a map[string]any")
				assert.Equal(t, "integer", additionalProps["type"])

				// Check nested struct field.
				nestedField, ok := properties["nested_struct"].(map[string]any)
				require.True(t, ok, "Expected nested_struct to be a map[string]any")
				assert.Contains(t, nestedField, "$ref")

				// Check pointer field (should be optional).
				pointerField, ok := properties["pointer_field"].(map[string]any)
				require.True(t, ok, "Expected pointer_field to be a map[string]any")
				assert.Equal(t, "string", pointerField["type"])

				// Check embedded fields are inlined.
				assert.Contains(t, properties, "embedded_value")
				assert.Contains(t, properties, "embedded_count")
			},
		},
		"nested complex struct": {
			reflectTarget: NestedComplexStruct{},
			packagePaths:  []string{},
			checkSchema: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				assert.Contains(t, schemaData, "$ref")
				assert.Contains(t, schemaData, "$defs")

				defs, ok := schemaData["$defs"].(map[string]any)
				require.True(t, ok, "Expected $defs to be a map[string]any")

				// Check that all nested types are defined.
				assert.Contains(t, defs, "NestedComplexStruct")
				assert.Contains(t, defs, "Level1Struct")
				assert.Contains(t, defs, "Level2Struct")
				assert.Contains(t, defs, "ItemStruct")

				// Check deeply nested structure.
				nestedDef, ok := defs["NestedComplexStruct"].(map[string]any)
				require.True(t, ok, "Expected NestedComplexStruct to be a map[string]any")
				properties, ok := nestedDef["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// Check Level1 field.
				level1Field, ok := properties["level1"].(map[string]any)
				require.True(t, ok, "Expected level1 to be a map[string]any")
				assert.Contains(t, level1Field, "$ref")

				// Check Items slice field.
				itemsField, ok := properties["items"].(map[string]any)
				require.True(t, ok, "Expected items to be a map[string]any")
				assert.Equal(t, "array", itemsField["type"])
				items, ok := itemsField["items"].(map[string]any)
				require.True(t, ok, "Expected items to be a map[string]any")
				assert.Contains(t, items, "$ref")
			},
		},
		"embedded struct": {
			reflectTarget: EmbeddedStruct{},
			packagePaths:  []string{},
			checkSchema: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				assert.Contains(t, schemaData, "$ref")
				assert.Contains(t, schemaData, "$defs")

				defs, ok := schemaData["$defs"].(map[string]any)
				require.True(t, ok, "Expected $defs to be a map[string]any")
				embeddedDef, ok := defs["EmbeddedStruct"].(map[string]any)
				require.True(t, ok, "Expected EmbeddedStruct to be a map[string]any")
				properties, ok := embeddedDef["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// Check embedded fields.
				assert.Contains(t, properties, "embedded_value")
				assert.Contains(t, properties, "embedded_count")

				embeddedValue, ok := properties["embedded_value"].(map[string]any)
				require.True(t, ok, "Expected embedded_value to be a map[string]any")
				assert.Equal(t, "string", embeddedValue["type"])

				embeddedCount, ok := properties["embedded_count"].(map[string]any)
				require.True(t, ok, "Expected embedded_count to be a map[string]any")
				assert.Equal(t, "integer", embeddedCount["type"])
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gen := schema.NewGenerator(tc.reflectTarget, tc.packagePaths...)
			gen.Tests = true
			result, err := gen.Generate()

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Validate that the result is valid JSON.
			var schemaData map[string]any
			err = json.Unmarshal(result, &schemaData)
			require.NoError(t, err)

			// Run custom schema checks.
			tc.checkSchema(t, schemaData)
		})
	}
}

func TestDefaultLookupCommentFunc_ComplexStructures(t *testing.T) {
	t.Parallel()

	commentMap := map[string]string{
		"schema_test.ComplexStruct":                "ComplexStruct demonstrates various Go type features.",
		"schema_test.ComplexStruct.BasicField":     "BasicField is a simple field.",
		"schema_test.ComplexStruct.SliceField":     "SliceField contains multiple values.",
		"schema_test.ComplexStruct.MapField":       "MapField contains key-value pairs.",
		"schema_test.ComplexStruct.NestedStruct":   "NestedStruct contains nested data.",
		"schema_test.ComplexStruct.PointerField":   "PointerField may be nil.",
		"schema_test.EmbeddedStruct":               "EmbeddedStruct provides embedded functionality.",
		"schema_test.EmbeddedStruct.EmbeddedValue": "EmbeddedValue is from an embedded struct.",
	}

	lookupFunc := schema.DefaultLookupCommentFunc(commentMap)

	tcs := map[string]struct {
		structType reflect.Type
		fieldName  string
		want       string
	}{
		"complex struct type comment": {
			structType: reflect.TypeOf(ComplexStruct{}),
			fieldName:  "",
			want:       "ComplexStruct demonstrates various Go type features.\n\nComplexStruct: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#ComplexStruct",
		},
		"complex struct basic field": {
			structType: reflect.TypeOf(ComplexStruct{}),
			fieldName:  "BasicField",
			want:       "BasicField is a simple field.\n\nComplexStruct.BasicField: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#ComplexStruct",
		},
		"complex struct slice field": {
			structType: reflect.TypeOf(ComplexStruct{}),
			fieldName:  "SliceField",
			want:       "SliceField contains multiple values.\n\nComplexStruct.SliceField: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#ComplexStruct",
		},
		"complex struct map field": {
			structType: reflect.TypeOf(ComplexStruct{}),
			fieldName:  "MapField",
			want:       "MapField contains key-value pairs.\n\nComplexStruct.MapField: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#ComplexStruct",
		},
		"embedded struct type comment": {
			structType: reflect.TypeOf(EmbeddedStruct{}),
			fieldName:  "",
			want:       "EmbeddedStruct provides embedded functionality.\n\nEmbeddedStruct: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#EmbeddedStruct",
		},
		"embedded struct field comment": {
			structType: reflect.TypeOf(EmbeddedStruct{}),
			fieldName:  "EmbeddedValue",
			want:       "EmbeddedValue is from an embedded struct.\n\nEmbeddedStruct.EmbeddedValue: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#EmbeddedStruct",
		},
		"missing comment for complex field": {
			structType: reflect.TypeOf(ComplexStruct{}),
			fieldName:  "UndocumentedField",
			want:       "ComplexStruct.UndocumentedField: https://pkg.go.dev/github.com/macropower/kat/pkg/schema_test#ComplexStruct",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := lookupFunc(tc.structType, tc.fieldName)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGenerator_Generate_WithPackagePaths(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		reflectTarget any
		packagePaths  []string
		expectError   bool
	}{
		"complex struct with current package path": {
			reflectTarget: ComplexStruct{},
			packagePaths:  []string{"github.com/macropower/kat/pkg/schema"},
			expectError:   false,
		},
		"nested struct with current package path": {
			reflectTarget: NestedComplexStruct{},
			packagePaths:  []string{"github.com/macropower/kat/pkg/schema"},
			expectError:   false,
		},
		"multiple package paths": {
			reflectTarget: ComplexStruct{},
			packagePaths: []string{
				"github.com/macropower/kat/pkg/schema",
				"github.com/macropower/kat/pkg/config",
			},
			expectError: false,
		},
		"empty package paths with complex struct": {
			reflectTarget: ComplexStruct{},
			packagePaths:  []string{},
			expectError:   false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gen := schema.NewGenerator(tc.reflectTarget, tc.packagePaths...)
			gen.Tests = true
			result, err := gen.Generate()

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)

				// Validate JSON structure.
				var schemaData map[string]any
				err = json.Unmarshal(result, &schemaData)
				require.NoError(t, err)

				// Basic validation that schema contains expected fields.
				if ref, hasRef := schemaData["$ref"]; hasRef {
					assert.NotNil(t, ref)
					assert.Contains(t, schemaData, "$defs")
				} else {
					assert.Contains(t, schemaData, "type")
				}
			}
		})
	}
}

func TestGenerator_EdgeCases(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		reflectTarget any
		description   string
		packagePaths  []string
	}{
		"primitive type": {
			reflectTarget: "string value",
			packagePaths:  []string{},
			description:   "Should handle primitive string type",
		},
		"slice of primitives": {
			reflectTarget: []int{},
			packagePaths:  []string{},
			description:   "Should handle slice of primitives",
		},
		"map of primitives": {
			reflectTarget: map[string]int{},
			packagePaths:  []string{},
			description:   "Should handle map of primitives",
		},
		"pointer to struct": {
			reflectTarget: &SimpleStruct{},
			packagePaths:  []string{},
			description:   "Should handle pointer to struct",
		},
		"slice of structs": {
			reflectTarget: []SimpleStruct{},
			packagePaths:  []string{},
			description:   "Should handle slice of structs",
		},
		"map of structs": {
			reflectTarget: map[string]SimpleStruct{},
			packagePaths:  []string{},
			description:   "Should handle map of structs",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gen := schema.NewGenerator(tc.reflectTarget, tc.packagePaths...)
			gen.Tests = true
			result, err := gen.Generate()

			// All these cases should produce valid schemas without errors.
			require.NoError(t, err, tc.description)
			assert.NotNil(t, result, tc.description)

			// Validate JSON structure.
			var schemaData map[string]any
			err = json.Unmarshal(result, &schemaData)
			require.NoError(t, err, "Generated schema should be valid JSON")
		})
	}
}

func TestGenerator_Generate_CommentExtractionPipeline(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		reflectTarget any
		checkComments func(t *testing.T, schemaData map[string]any)
		packagePaths  []string
	}{
		"TestStruct with package path - full pipeline": {
			reflectTarget: TestStruct{},
			packagePaths:  []string{"github.com/macropower/kat/pkg/schema"},
			checkComments: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				// TestStruct should generate a direct schema.
				properties, ok := schemaData["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// The Name field should have its actual comment extracted.
				nameField, ok := properties["name"].(map[string]any)
				require.True(t, ok, "Expected nameField to be a map[string]any")
				description, hasDescription := nameField["description"]

				if hasDescription {
					t.Logf("Name field description: %s", description)
					descStr, ok := description.(string)
					require.True(t, ok, "Expected description to be a string")
					// This should contain the actual comment, not just the URL.
					if !strings.Contains(descStr, "Name is the name of the test") {
						t.Errorf("Name field description missing actual comment, got: %s", descStr)
					}
				} else {
					t.Error("Name field is missing description comment")
				}

				// The Age field should also have its comment.
				ageField, ok := properties["age"].(map[string]any)
				require.True(t, ok, "Expected ageField to be a map[string]any")
				ageDesc, hasAgeDesc := ageField["description"]

				if hasAgeDesc {
					t.Logf("Age field description: %s", ageDesc)
					ageDescStr, ok := ageDesc.(string)
					require.True(t, ok, "Expected ageDesc to be a string")
					if !strings.Contains(ageDescStr, "Age is the age of the person") {
						t.Errorf("Age field description missing actual comment, got: %s", ageDescStr)
					}
				} else {
					t.Error("Age field is missing description comment")
				}
			},
		},
		"ComplexStruct with package path - full pipeline": {
			reflectTarget: ComplexStruct{},
			packagePaths:  []string{"github.com/macropower/kat/pkg/schema"},
			checkComments: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				// ComplexStruct generates a direct schema (not using $defs).
				properties, ok := schemaData["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// Check that BasicField has its comment.
				basicField, ok := properties["basic_field"].(map[string]any)
				require.True(t, ok, "Expected basicField to be a map[string]any")
				basicDesc, hasBasicDesc := basicField["description"]

				if hasBasicDesc {
					t.Logf("BasicField description: %s", basicDesc)
					basicDescStr, ok := basicDesc.(string)
					require.True(t, ok, "Expected basicDesc to be a string")
					if !strings.Contains(basicDescStr, "BasicField is a simple string field") {
						t.Errorf("BasicField description missing actual comment, got: %s", basicDescStr)
					}
				} else {
					t.Error("BasicField is missing description comment")
				}

				// Check that SliceField has its comment.
				sliceField, ok := properties["slice_field"].(map[string]any)
				require.True(t, ok, "Expected sliceField to be a map[string]any")
				sliceDesc, hasSliceDesc := sliceField["description"]

				if hasSliceDesc {
					t.Logf("SliceField description: %s", sliceDesc)
					sliceDescStr, ok := sliceDesc.(string)
					require.True(t, ok, "Expected sliceDesc to be a string")
					if !strings.Contains(sliceDescStr, "SliceField contains a slice of strings") {
						t.Errorf("SliceField description missing actual comment, got: %s", sliceDescStr)
					}
				} else {
					t.Error("SliceField is missing description comment")
				}
			},
		},
		"same struct without package path - no comment extraction": {
			reflectTarget: TestStruct{},
			packagePaths:  []string{}, // No package paths = no comment extraction
			checkComments: func(t *testing.T, schemaData map[string]any) {
				t.Helper()

				// When using $defs structure, check the definition.
				defs, ok := schemaData["$defs"].(map[string]any)
				require.True(t, ok, "Expected $defs to be a map[string]any")
				testStructDef, ok := defs["TestStruct"].(map[string]any)
				require.True(t, ok, "Expected TestStruct to be a map[string]any")
				properties, ok := testStructDef["properties"].(map[string]any)
				require.True(t, ok, "Expected properties to be a map[string]any")

				// Without package paths, fields should have no descriptions at all.
				nameField, ok := properties["name"].(map[string]any)
				require.True(t, ok, "Expected nameField to be a map[string]any")
				_, hasDescription := nameField["description"]

				if hasDescription {
					t.Error("Unexpected description found when no package paths provided - should have no descriptions without comment extraction")
				} else {
					t.Log("Correctly no description when no package paths provided")
				}
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gen := schema.NewGenerator(tc.reflectTarget, tc.packagePaths...)
			gen.Tests = true
			result, err := gen.Generate()

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Validate that the result is valid JSON.
			var schemaData map[string]any
			err = json.Unmarshal(result, &schemaData)
			require.NoError(t, err)

			// Print the generated schema for debugging.
			t.Logf("Generated schema: %s", string(result))

			// Run custom comment checks.
			tc.checkComments(t, schemaData)
		})
	}
}
