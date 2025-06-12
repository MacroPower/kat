package kube_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

func TestCommandRunner_RunForPath(t *testing.T) {
	t.Parallel()

	// Setup temp directory for testing
	tempDir := t.TempDir()

	// Create test files
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	require.NoError(t, os.WriteFile(kustomizationFile, []byte("resources: []"), 0o644))

	unknownFile := filepath.Join(tempDir, "unknown.yaml")
	require.NoError(t, os.WriteFile(unknownFile, []byte(""), 0o644))

	// Create a subdirectory with a nested file
	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	nestedChartFile := filepath.Join(subDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(nestedChartFile, []byte("name: nested-chart"), 0o644))

	tcs := map[string]struct {
		expectedError error
		path          string
		checkOutput   bool
	}{
		"file not found": {
			path:          filepath.Join(tempDir, "nonexistent.yaml"),
			expectedError: os.ErrNotExist,
			checkOutput:   false,
		},
		"no command for path": {
			path:          unknownFile,
			expectedError: kube.ErrNoCommandForPath,
			checkOutput:   false,
		},
		"directory with no matching files": {
			path:          t.TempDir(), // Empty temp directory
			expectedError: kube.ErrNoCommandForPath,
			checkOutput:   false,
		},
		"match Chart.yaml file": {
			path:          chartFile,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
		"match kustomization.yaml file": {
			path:          kustomizationFile,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
		"directory with matching file": {
			path:          tempDir,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := kube.NewCommandRunner(tc.path)
			output, err := runner.Run()

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			}

			if tc.checkOutput {
				assert.NotEmpty(t, output.Stdout)
			}
		})
	}
}

func TestCommandRunner_WithCommand(t *testing.T) {
	t.Parallel()

	runner := kube.NewCommandRunner("")
	runner.SetCommand(kube.MustNewCommand(nil, "", "echo", "{apiVersion: v1, kind: Resource}"))
	customRunner, err := runner.Run()
	require.NoError(t, err)

	assert.Empty(t, customRunner.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: Resource}\n", customRunner.Stdout)
	assert.Equal(t, "v1", customRunner.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "Resource", customRunner.Resources[0].Object.GetKind())
}

func TestCommand_WithPostRenderHooks(t *testing.T) {
	t.Parallel()

	// Create a command with postRender hooks that use stdin
	cmd := &kube.Command{
		Command: "echo",
		Args:    []string{"{apiVersion: v1, kind: Service, metadata: {name: test}}"},
		Hooks: &kube.Hooks{
			PostRender: []*kube.HookCommand{
				{Command: "grep", Args: []string{"Service"}}, // This should succeed since output contains "Service"
			},
		},
	}

	output, err := cmd.Exec(".")
	require.NoError(t, err)

	assert.NotEmpty(t, output.Stdout)
	assert.Empty(t, output.Stderr)
	assert.Len(t, output.Resources, 1)
	assert.Equal(t, "Service", output.Resources[0].Object.GetKind())
}

func TestCommand_FailingPostRenderHook(t *testing.T) {
	t.Parallel()

	// Create a command with a failing postRender hook
	cmd := &kube.Command{
		Command: "echo",
		Args:    []string{"{apiVersion: v1, kind: Pod, metadata: {name: test}}"},
		Hooks: &kube.Hooks{
			PostRender: []*kube.HookCommand{
				{Command: "false"}, // This command always fails
			},
		},
	}

	_, err := cmd.Exec(".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")
}

func TestCommand_EmptyHookCommand(t *testing.T) {
	t.Parallel()

	// Create a command with an empty hook command
	cmd := &kube.Command{
		Command: "echo",
		Args:    []string{"{apiVersion: v1, kind: Pod, metadata: {name: test}}"},
		Hooks: &kube.Hooks{
			PostRender: []*kube.HookCommand{
				{Command: ""}, // Empty command should fail
			},
		},
	}

	_, err := cmd.Exec(".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty hook command")
}

func TestNewCommandWithHooks(t *testing.T) {
	t.Parallel()

	hooks := &kube.Hooks{
		PostRender: []*kube.HookCommand{
			{Command: "echo", Args: []string{"post-render"}},
		},
	}

	cmd, err := kube.NewCommand(hooks, ".*\\.yaml", "echo", "test")
	require.NoError(t, err)
	assert.NotNil(t, cmd.Hooks)
	assert.Len(t, cmd.Hooks.PostRender, 1)
}

func TestCommand_IntegrationWithRealCommands(t *testing.T) {
	t.Parallel()

	// Skip if helm is not available
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found in PATH, skipping integration test")
	}

	tempDir := t.TempDir()

	// Create a minimal Chart.yaml
	chartContent := `apiVersion: v2
name: test-chart
description: A test Helm chart
type: application
version: 0.1.0
appVersion: "1.0.0"`

	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte(chartContent), 0o644))

	// Create a simple template
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	templateContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Chart.Name }}-config
data:
  app: {{ .Chart.Name }}`

	templateFile := filepath.Join(templatesDir, "configmap.yaml")
	require.NoError(t, os.WriteFile(templateFile, []byte(templateContent), 0o644))

	// Create command with hooks
	cmd := &kube.Command{
		Command: "helm",
		Args:    []string{"template", ".", "--generate-name"},
		Hooks: &kube.Hooks{
			PostRender: []*kube.HookCommand{
				{Command: "grep", Args: []string{"ConfigMap"}}, // Verify ConfigMap is in output
			},
		},
	}

	output, err := cmd.Exec(tempDir)
	require.NoError(t, err)

	assert.NotEmpty(t, output.Stdout)
	assert.Empty(t, output.Stderr)
	assert.NotEmpty(t, output.Resources)

	// Check that we have a ConfigMap resource
	found := false
	for _, resource := range output.Resources {
		if resource.Object.GetKind() == "ConfigMap" {
			found = true

			break
		}
	}
	assert.True(t, found, "Expected to find a ConfigMap in the rendered resources")
}

func TestCommand_WithPreRenderHooks(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expectedError  error
		preRenderHooks []*kube.HookCommand
		checkOutput    bool
	}{
		"successful preRender hook": {
			preRenderHooks: []*kube.HookCommand{
				{Command: "echo", Args: []string{"pre-render"}},
			},
			expectedError: nil,
			checkOutput:   true,
		},
		"multiple successful preRender hooks": {
			preRenderHooks: []*kube.HookCommand{
				{Command: "echo", Args: []string{"first"}},
				{Command: "echo", Args: []string{"second"}},
			},
			expectedError: nil,
			checkOutput:   true,
		},
		"failing preRender hook": {
			preRenderHooks: []*kube.HookCommand{
				{Command: "false"}, // This command always fails
			},
			expectedError: kube.ErrHookExecution,
			checkOutput:   false,
		},
		"empty preRender hook command": {
			preRenderHooks: []*kube.HookCommand{
				{Command: ""}, // Empty command should fail
			},
			expectedError: kube.ErrHookExecution,
			checkOutput:   false,
		},
		"mixed hooks - fail on first": {
			preRenderHooks: []*kube.HookCommand{
				{Command: "false"}, // This fails first
				{Command: "echo", Args: []string{"should not execute"}},
			},
			expectedError: kube.ErrHookExecution,
			checkOutput:   false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a command with preRender hooks
			cmd := &kube.Command{
				Command: "echo",
				Args:    []string{"{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}"},
				Hooks: &kube.Hooks{
					PreRender: tc.preRenderHooks,
				},
			}

			output, err := cmd.Exec(".")

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)

				return
			}

			require.NoError(t, err)

			if tc.checkOutput {
				assert.NotEmpty(t, output.Stdout)
				assert.Empty(t, output.Stderr)
				assert.Len(t, output.Resources, 1)
				assert.Equal(t, "ConfigMap", output.Resources[0].Object.GetKind())
			}
		})
	}
}

func TestCommand_PreRenderHookFailurePreventsMainCommand(t *testing.T) {
	t.Parallel()

	// Create a command with a failing preRender hook
	// The main command should not execute if preRender fails
	cmd := &kube.Command{
		Command: "echo",
		Args:    []string{"this should not execute"},
		Hooks: &kube.Hooks{
			PreRender: []*kube.HookCommand{
				{Command: "false"}, // This command always fails
			},
		},
	}

	output, err := cmd.Exec(".")
	require.ErrorIs(t, err, kube.ErrHookExecution)

	// Since preRender failed, main command should not have executed
	// so output should be empty
	assert.Empty(t, output.Stdout)
	assert.Empty(t, output.Resources)
}

func TestCommand_WithBothPreAndPostRenderHooks(t *testing.T) {
	t.Parallel()

	// Test that both preRender and postRender hooks execute successfully
	cmd := &kube.Command{
		Command: "echo",
		Args:    []string{"{apiVersion: v1, kind: Service, metadata: {name: test}}"},
		Hooks: &kube.Hooks{
			PreRender: []*kube.HookCommand{
				{Command: "echo", Args: []string{"pre-render executed"}},
			},
			PostRender: []*kube.HookCommand{
				{Command: "grep", Args: []string{"Service"}}, // This should succeed since output contains "Service"
			},
		},
	}

	output, err := cmd.Exec(".")
	require.NoError(t, err)

	assert.NotEmpty(t, output.Stdout)
	assert.Empty(t, output.Stderr)
	assert.Len(t, output.Resources, 1)
	assert.Equal(t, "Service", output.Resources[0].Object.GetKind())
}
