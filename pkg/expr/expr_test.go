package expr_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/expr"
)

func TestCELFilepathFunctions(t *testing.T) {
	t.Parallel()

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tests := []struct {
		name       string
		expression string
		files      []string
		expected   bool
	}{
		{
			name:       "pathBase with in operator - exists",
			expression: `files.exists(f, pathBase(f) in ["Chart.yaml", "values.yaml"])`,
			files: []string{
				"/k8s/Chart.yaml",
				"/k8s/values.yaml",
				"/k8s/deployment.yaml",
				"/k8s/service.yaml",
			},
			expected: true,
		},
		{
			name:       "pathExt with in operator - exists",
			expression: `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			files: []string{
				"/k8s/deployment.yaml",
				"/k8s/service.yml",
				"/k8s/configmap.json",
				"/k8s/Chart.yaml",
			},
			expected: true,
		},
		{
			name:       "pathDir with contains - exists",
			expression: `files.exists(f, pathDir(f).contains("/templates"))`,
			files: []string{
				"/k8s/templates/deployment.yaml",
				"/k8s/templates/service.yaml",
				"/k8s/Chart.yaml",
				"/k8s/values.yaml",
			},
			expected: true,
		},
		{
			name:       "pathBase equality check - exists",
			expression: `files.exists(f, pathBase(f) == "Chart.yaml")`,
			files: []string{
				"/k8s/Chart.yaml",
				"/k8s/values.yaml",
				"/k8s/templates/Chart.yaml",
			},
			expected: true,
		},
		{
			name:       "combined filepath functions - exists",
			expression: `files.exists(f, pathExt(f) in [".yaml", ".yml"] && !pathBase(f).matches(".*test.*"))`,
			files: []string{
				"/k8s/deployment.yaml",
				"/k8s/service.yml",
				"/k8s/test-config.yaml",
				"/k8s/readme.txt",
			},
			expected: true,
		},
		{
			name:       "no matches - false",
			expression: `files.exists(f, pathBase(f) == "nonexistent.yaml")`,
			files: []string{
				"/k8s/deployment.yaml",
				"/k8s/service.yaml",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Compile the CEL expression
			ast, issues := env.Compile(test.expression)
			require.NoError(t, issues.Err())

			program, err := env.Program(ast)
			require.NoError(t, err)

			// Create input data
			fileList := types.NewStringList(types.DefaultTypeAdapter, test.files)
			vars := map[string]any{
				"files": fileList,
				"dir":   "/k8s",
			}

			// Evaluate the expression
			result, _, err := program.Eval(vars)
			require.NoError(t, err)

			boolResult, ok := result.Value().(bool)
			require.True(t, ok, "result should be a boolean")
			require.Equal(t, test.expected, boolResult)
		})
	}
}

func TestCELYamlPathFunction(t *testing.T) {
	t.Parallel()

	// Create temporary directory with test files
	tempDir := t.TempDir()

	// Create Chart.yaml with apiVersion v2
	chartContent := `apiVersion: v2
name: test-chart
version: 1.0.0
appVersion: "1.0"
description: A test Helm chart
`
	chartPath := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartPath, []byte(chartContent), 0o644))

	// Create values.yaml with some nested structure
	valuesContent := `replicaCount: 1
image:
  repository: nginx
  tag: "1.16.0"
  pullPolicy: IfNotPresent
service:
  type: ClusterIP
  port: 80
`
	valuesPath := filepath.Join(tempDir, "values.yaml")
	require.NoError(t, os.WriteFile(valuesPath, []byte(valuesContent), 0o644))

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tests := []struct {
		name       string
		expression string
		files      []string
		expected   bool
	}{
		{
			name:       "yamlPath function with valid apiVersion",
			expression: `files.exists(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2")`,
			files:      []string{chartPath, valuesPath},
			expected:   true,
		},
		{
			name:       "yamlPath function with nested path",
			expression: `files.exists(f, pathBase(f) == "values.yaml" && yamlPath(f, "$.image.repository") == "nginx")`,
			files:      []string{chartPath, valuesPath},
			expected:   true,
		},
		{
			name:       "yamlPath function with non-existent path",
			expression: `files.exists(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.nonExistent") != null)`,
			files:      []string{chartPath},
			expected:   false,
		},
		{
			name:       "yamlPath function with numeric value",
			expression: `files.exists(f, pathBase(f) == "values.yaml" && yamlPath(f, "$.replicaCount") == 1)`,
			files:      []string{chartPath, valuesPath},
			expected:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Compile the CEL expression
			ast, issues := env.Compile(test.expression)
			require.NoError(t, issues.Err())

			program, err := env.Program(ast)
			require.NoError(t, err)

			// Create input data
			fileList := types.NewStringList(types.DefaultTypeAdapter, test.files)
			vars := map[string]any{
				"files": fileList,
				"dir":   tempDir,
			}

			// Evaluate the expression
			result, _, err := program.Eval(vars)
			require.NoError(t, err)

			boolResult, ok := result.Value().(bool)
			require.True(t, ok, "result should be a boolean")
			require.Equal(t, test.expected, boolResult)
		})
	}
}

func TestConvertToCELValue(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input    any
		expected any
		isNull   bool
	}{
		"nil value": {
			input:  nil,
			isNull: true,
		},
		"bool true": {
			input:    true,
			expected: true,
		},
		"bool false": {
			input:    false,
			expected: false,
		},
		"int": {
			input:    42,
			expected: int64(42),
		},
		"int8": {
			input:    int8(42),
			expected: int64(42),
		},
		"int16": {
			input:    int16(42),
			expected: int64(42),
		},
		"int32": {
			input:    int32(42),
			expected: int64(42),
		},
		"int64": {
			input:    int64(42),
			expected: int64(42),
		},
		"uint": {
			input:    uint(42),
			expected: int64(42),
		},
		"uint overflow": {
			input:    uint(math.MaxUint64),
			expected: float64(math.MaxUint64),
		},
		"uint8": {
			input:    uint8(42),
			expected: int64(42),
		},
		"uint16": {
			input:    uint16(42),
			expected: int64(42),
		},
		"uint32": {
			input:    uint32(42),
			expected: int64(42),
		},
		"uint64": {
			input:    uint64(42),
			expected: int64(42),
		},
		"uint64 overflow": {
			input:    uint64(math.MaxUint64),
			expected: float64(math.MaxUint64),
		},
		"float32": {
			input:    float32(3.14),
			expected: float64(float32(3.14)),
		},
		"float64": {
			input:    3.14159,
			expected: 3.14159,
		},
		"string": {
			input:    "hello world",
			expected: "hello world",
		},
		"unsupported type": {
			input:  complex(1, 2),
			isNull: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := expr.ConvertToCELValue(tc.input)

			if tc.isNull {
				assert.Equal(t, types.NullValue, result)

				return
			}

			switch expected := tc.expected.(type) {
			case bool:
				boolVal, ok := result.Value().(bool)
				require.True(t, ok)
				assert.Equal(t, expected, boolVal)
			case int64:
				intVal, ok := result.Value().(int64)
				require.True(t, ok)
				assert.Equal(t, expected, intVal)
			case float64:
				floatVal, ok := result.Value().(float64)
				require.True(t, ok)
				assert.InDelta(t, expected, floatVal, 0.01)
			case string:
				strVal, ok := result.Value().(string)
				require.True(t, ok)
				assert.Equal(t, expected, strVal)
			}
		})
	}
}

func TestConvertToCELValue_Slice(t *testing.T) {
	t.Parallel()

	input := []any{1, "hello", true, nil}
	result := expr.ConvertToCELValue(input)

	// The result should not be null and should have the correct type
	assert.NotEqual(t, types.NullValue, result)
	assert.Equal(t, "list", result.Type().TypeName())
}

func TestConvertToCELValue_MapAnyAny(t *testing.T) {
	t.Parallel()

	input := map[any]any{
		"key1": "value1",
		42:     "value2",
	}
	result := expr.ConvertToCELValue(input)

	// The result should not be null and should have the correct type
	assert.NotEqual(t, types.NullValue, result)
	assert.Equal(t, "map", result.Type().TypeName())
}

func TestConvertToCELValue_MapStringAny(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"name":    "test",
		"count":   42,
		"enabled": true,
		"nested": map[string]any{
			"inner": "value",
		},
	}
	result := expr.ConvertToCELValue(input)

	// The result should not be null and should have the correct type
	assert.NotEqual(t, types.NullValue, result)
	assert.Equal(t, "map", result.Type().TypeName())
}

func TestCELErrorHandling(t *testing.T) {
	t.Parallel()

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tcs := map[string]struct {
		vars       map[string]any
		expression string
		shouldErr  bool
	}{
		"pathBase with invalid input": {
			expression: `pathBase(42)`,
			vars:       map[string]any{},
			shouldErr:  true,
		},
		"pathDir with invalid input": {
			expression: `pathDir(true)`,
			vars:       map[string]any{},
			shouldErr:  true,
		},
		"pathExt with invalid input": {
			expression: `pathExt([])`,
			vars:       map[string]any{},
			shouldErr:  true,
		},
		"yamlPath with invalid file path": {
			expression: `yamlPath(123, "$.test")`,
			vars:       map[string]any{},
			shouldErr:  true,
		},
		"yamlPath with invalid yaml path": {
			expression: `yamlPath("/test.yaml", 456)`,
			vars:       map[string]any{},
			shouldErr:  true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ast, issues := env.Compile(tc.expression)
			if issues.Err() != nil {
				if tc.shouldErr {
					return // Expected compilation error
				}
				require.NoError(t, issues.Err())
			}

			program, err := env.Program(ast)
			require.NoError(t, err)

			result, _, err := program.Eval(tc.vars)
			if tc.shouldErr {
				// Check if result is an error or if evaluation failed
				if err != nil {
					return
				}
				// Check if result contains error
				if errVal, ok := result.(*types.Err); ok {
					assert.Contains(t, errVal.Error(), "invalid")

					return
				}
				t.Error("Expected an error but got none")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestYamlPathErrorCases(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an invalid YAML file
	invalidYAMLPath := filepath.Join(tempDir, "invalid.yaml")
	require.NoError(t, os.WriteFile(invalidYAMLPath, []byte("invalid: yaml: content: ["), 0o644))

	// Create a valid YAML file for testing invalid paths
	validYAMLPath := filepath.Join(tempDir, "valid.yaml")
	validContent := `name: test
version: 1.0`
	require.NoError(t, os.WriteFile(validYAMLPath, []byte(validContent), 0o644))

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tcs := map[string]struct {
		expression string
		files      []string
	}{
		"non-existent file": {
			expression: `yamlPath("/non/existent/file.yaml", "$.name")`,
			files:      []string{},
		},
		"invalid YAML path syntax": {
			expression: `yamlPath("` + validYAMLPath + `", "invalid[path")`,
			files:      []string{validYAMLPath},
		},
		"path not found in YAML": {
			expression: `yamlPath("` + validYAMLPath + `", "$.nonexistent.field")`,
			files:      []string{validYAMLPath},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ast, issues := env.Compile(tc.expression)
			require.NoError(t, issues.Err())

			program, err := env.Program(ast)
			require.NoError(t, err)

			vars := map[string]any{
				"files": types.NewStringList(types.DefaultTypeAdapter, tc.files),
				"dir":   tempDir,
			}

			result, _, err := program.Eval(vars)
			require.NoError(t, err)

			// All these cases should return null instead of erroring
			assert.Equal(t, types.NullValue, result)
		})
	}
}

func TestCELComplexDataTypes(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a complex YAML file
	complexContent := `
metadata:
  name: test-app
  labels:
    app: test
    version: "1.0"
spec:
  replicas: 3
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: https
      port: 443
      targetPort: 8443
  config:
    enabled: true
    timeout: 30.5
    items:
      - item1
      - item2
      - item3
`
	complexPath := filepath.Join(tempDir, "complex.yaml")
	require.NoError(t, os.WriteFile(complexPath, []byte(complexContent), 0o644))

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tcs := map[string]struct {
		expected   any
		expression string
	}{
		"string value": {
			expression: `yamlPath("` + complexPath + `", "$.metadata.name")`,
			expected:   "test-app",
		},
		"integer value": {
			expression: `yamlPath("` + complexPath + `", "$.spec.replicas")`,
			expected:   int64(3),
		},
		"boolean value": {
			expression: `yamlPath("` + complexPath + `", "$.spec.config.enabled")`,
			expected:   true,
		},
		"float value": {
			expression: `yamlPath("` + complexPath + `", "$.spec.config.timeout")`,
			expected:   30.5,
		},
		"array element": {
			expression: `yamlPath("` + complexPath + `", "$.spec.config.items[0]")`,
			expected:   "item1",
		},
		"nested object value": {
			expression: `yamlPath("` + complexPath + `", "$.spec.ports[0].port")`,
			expected:   int64(80),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ast, issues := env.Compile(tc.expression)
			require.NoError(t, issues.Err())

			program, err := env.Program(ast)
			require.NoError(t, err)

			vars := map[string]any{
				"files": types.NewStringList(types.DefaultTypeAdapter, []string{complexPath}),
				"dir":   tempDir,
			}

			result, _, err := program.Eval(vars)
			require.NoError(t, err)

			switch expected := tc.expected.(type) {
			case string:
				strVal, ok := result.Value().(string)
				require.True(t, ok)
				assert.Equal(t, expected, strVal)
			case int64:
				intVal, ok := result.Value().(int64)
				require.True(t, ok)
				assert.Equal(t, expected, intVal)
			case bool:
				boolVal, ok := result.Value().(bool)
				require.True(t, ok)
				assert.Equal(t, expected, boolVal)
			case float64:
				floatVal, ok := result.Value().(float64)
				require.True(t, ok)
				assert.InDelta(t, expected, floatVal, 0.01)
			}
		})
	}
}

func TestCreateEnvironmentError(t *testing.T) {
	t.Parallel()

	// This test ensures we can create an environment without errors
	env, err := expr.CreateEnvironment()
	require.NoError(t, err)
	assert.NotNil(t, env)
}

func TestCELPathFunctionEdgeCases(t *testing.T) {
	t.Parallel()

	env, err := expr.CreateEnvironment()
	require.NoError(t, err)

	tcs := map[string]struct {
		expression string
		expected   string
	}{
		"pathBase root": {
			expression: `pathBase("/")`,
			expected:   "/",
		},
		"pathBase empty": {
			expression: `pathBase("")`,
			expected:   ".",
		},
		"pathDir root": {
			expression: `pathDir("/")`,
			expected:   "/",
		},
		"pathDir empty": {
			expression: `pathDir("")`,
			expected:   ".",
		},
		"pathExt no extension": {
			expression: `pathExt("/path/file")`,
			expected:   "",
		},
		"pathExt multiple extensions": {
			expression: `pathExt("/path/file.tar.gz")`,
			expected:   ".gz",
		},
		"pathExt hidden file": {
			expression: `pathExt("/path/.hidden")`,
			expected:   ".hidden",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ast, issues := env.Compile(tc.expression)
			require.NoError(t, issues.Err())

			program, err := env.Program(ast)
			require.NoError(t, err)

			result, _, err := program.Eval(map[string]any{})
			require.NoError(t, err)

			strVal, ok := result.Value().(string)
			require.True(t, ok)
			assert.Equal(t, tc.expected, strVal)
		})
	}
}
