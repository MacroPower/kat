package expr_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/expr"
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
