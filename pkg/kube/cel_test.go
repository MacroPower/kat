package kube_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

func TestCELFilepathFunctions(t *testing.T) {
	t.Parallel()

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

			rule, err := kube.NewRule(test.name, test.expression, "test")
			require.NoError(t, err)

			matches := rule.MatchFiles("/k8s", test.files)
			require.Equal(t, test.expected, matches)
		})
	}
}

func TestCELFilepathFunctionsInProfile(t *testing.T) {
	t.Parallel()

	// Test that filepath functions work in profile source expressions too
	profile, err := kube.NewProfile("kubectl",
		kube.WithArgs("apply", "-f", "-"),
		kube.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"] && !pathBase(f).matches(".*test.*"))`),
	)
	require.NoError(t, err)

	files := []string{
		"/k8s/deployment.yaml",
		"/k8s/service.yml",
		"/k8s/test-config.yaml",
		"/k8s/readme.txt",
	}

	matches, result := profile.MatchFiles("/k8s", files)
	expected := []string{"/k8s/deployment.yaml", "/k8s/service.yml"}
	require.True(t, matches)
	require.ElementsMatch(t, expected, result)
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

			rule, err := kube.NewRule(test.name, test.expression, "test")
			require.NoError(t, err)

			matches := rule.MatchFiles(tempDir, test.files)
			require.Equal(t, test.expected, matches)
		})
	}
}
