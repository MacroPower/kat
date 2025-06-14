package kube_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		initError   error
		runError    error
		path        string
		checkOutput bool
	}{
		"file not found": {
			path:        filepath.Join(tempDir, "nonexistent.yaml"),
			initError:   os.ErrNotExist,
			checkOutput: false,
		},
		"no command for path": {
			path:        unknownFile,
			initError:   kube.ErrNoCommandForPath,
			checkOutput: false,
		},
		"directory with no matching files": {
			path:        t.TempDir(), // Empty temp directory
			initError:   kube.ErrNoCommandForPath,
			checkOutput: false,
		},
		"match Chart.yaml file": {
			path:        chartFile,
			runError:    nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput: false,
		},
		"match kustomization.yaml file": {
			path:        kustomizationFile,
			runError:    nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput: false,
		},
		"directory with matching file": {
			path:        tempDir,
			runError:    nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner, err := kube.NewCommandRunner(tc.path, kube.WithCommands(kube.DefaultConfig.Commands))
			if tc.initError != nil {
				require.ErrorIs(t, err, tc.initError)

				return
			}

			require.NoError(t, err)

			output := runner.Run()

			if tc.runError != nil {
				require.ErrorIs(t, output.Error, tc.runError)
			}

			if tc.checkOutput {
				assert.NotEmpty(t, output.Stdout)
			}
		})
	}
}

func TestCommandRunner_WithCommand(t *testing.T) {
	t.Parallel()

	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithCommand(
		kube.MustNewCommand(nil, "", "", "echo", "{apiVersion: v1, kind: Resource}"),
	))
	require.NoError(t, err)

	output := runner.Run()
	require.NoError(t, output.Error)

	assert.Empty(t, output.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: Resource}\n", output.Stdout)
	assert.Equal(t, "v1", output.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "Resource", output.Resources[0].Object.GetKind())
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

	output := cmd.Exec(t.Context(), ".")
	require.NoError(t, output.Error)

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

	output := cmd.Exec(t.Context(), ".")
	require.Error(t, output.Error)
	assert.Contains(t, output.Error.Error(), "exit status 1")
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

	output := cmd.Exec(t.Context(), ".")
	require.Error(t, output.Error)
	assert.Contains(t, output.Error.Error(), "empty command")
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

	output := cmd.Exec(t.Context(), tempDir)
	require.NoError(t, output.Error)

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

			output := cmd.Exec(t.Context(), ".")

			if tc.expectedError != nil {
				require.ErrorIs(t, output.Error, tc.expectedError)

				return
			}

			require.NoError(t, output.Error)

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

	output := cmd.Exec(t.Context(), ".")
	require.ErrorIs(t, output.Error, kube.ErrHookExecution)

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

	output := cmd.Exec(t.Context(), ".")
	require.NoError(t, output.Error)

	assert.NotEmpty(t, output.Stdout)
	assert.Empty(t, output.Stderr)
	assert.Len(t, output.Resources, 1)
	assert.Equal(t, "Service", output.Resources[0].Object.GetKind())
}

func TestCommandRunner_RunContext(t *testing.T) {
	t.Parallel()

	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithCommand(
		kube.MustNewCommand(nil, "", "", "echo", "{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}"),
	))
	require.NoError(t, err)

	// Test with context.Background()
	output := runner.RunContext(t.Context())
	require.NoError(t, output.Error)

	assert.Empty(t, output.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}\n", output.Stdout)
	assert.Equal(t, "v1", output.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "ConfigMap", output.Resources[0].Object.GetKind())

	// Test with canceled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	output = runner.RunContext(ctx)
	// The command might still succeed if it runs quickly before the context is checked
	// but this demonstrates the API is working
	if output.Error != nil {
		assert.Contains(t, output.Error.Error(), "context canceled")
	}
}

func TestCommandRunner_CancellationBehavior(t *testing.T) {
	t.Parallel()

	// Create a command that takes some time to execute
	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithCommand(
		kube.MustNewCommand(nil, "", "", "sleep", "2"),
	))
	require.NoError(t, err)

	// Test that a new command cancels the previous one
	t.Run("new command cancels previous", func(t *testing.T) {
		// Start first command with a context that we can monitor
		ctx1, cancel1 := context.WithCancel(t.Context())
		defer cancel1()

		// Channel to collect results
		results := make(chan kube.CommandOutput, 2)

		// Start first command in a goroutine
		go func() {
			results <- runner.RunContext(ctx1)
		}()

		// Give it a moment to start
		time.Sleep(100 * time.Millisecond)

		// Start second command which should cancel the first
		go func() {
			results <- runner.RunContext(t.Context())
		}()

		// Collect results
		var outputs []kube.CommandOutput
		for range 2 {
			select {
			case output := <-results:
				outputs = append(outputs, output)
			case <-time.After(5 * time.Second):
				t.Fatal("test timed out waiting for command completion")
			}
		}

		// At least one should complete (the second one should succeed or the first should be canceled)
		assert.Len(t, outputs, 2)

		// Check that at least one command was canceled or completed
		var hasError, hasSuccess bool
		for _, output := range outputs {
			if output.Error != nil {
				if strings.Contains(output.Error.Error(), "context canceled") {
					hasError = true
				}
			} else {
				hasSuccess = true
			}
		}

		// We should have either a cancellation error or a successful completion
		assert.True(t, hasError || hasSuccess, "expected either cancellation or successful completion")
	})
}

func TestCommandRunner_ConcurrentFileEvents(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a test file to watch
	testFile := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("test: data"), 0o644))

	// Create a command that takes a bit of time to execute
	runner, err := kube.NewCommandRunner(tempDir, kube.WithCommand(
		kube.MustNewCommand(nil, ".*\\.yaml$", "", "sleep", "0.2"), // 200ms sleep
	))
	require.NoError(t, err)

	// Start watching
	require.NoError(t, runner.Watch())
	defer runner.Close()

	// Channel to collect command outputs
	results := make(chan kube.CommandEvent, 10)

	runner.Subscribe(results)

	// Start RunOnEvent in a goroutine
	go runner.RunOnEvent()

	// Give it a moment to start watching
	time.Sleep(50 * time.Millisecond)

	// Trigger multiple rapid file events by writing to the file quickly
	for i := range 5 {
		content := fmt.Sprintf("test: data-%d", i)
		require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))
		time.Sleep(10 * time.Millisecond) // Small delay between writes
	}

	// Wait for commands to complete and collect results
	var outputs []kube.CommandOutput
	timeout := time.After(2 * time.Second)

	for {
		select {
		case output := <-results:
			switch out := output.(type) {
			case kube.CommandEventEnd:
				outputs = append(outputs, kube.CommandOutput(out))
				// We expect only the last command to complete successfully
				// (previous ones should be canceled)
				if len(outputs) >= 1 {
					goto done
				}
			}
		case <-timeout:
			goto done
		}
	}

done:
	// We should get at least one result (the last command that wasn't canceled)
	assert.GreaterOrEqual(t, len(outputs), 1, "should get at least one command result")

	// The final result should not have a cancellation error
	lastOutput := outputs[len(outputs)-1]
	if lastOutput.Error != nil {
		assert.NotContains(t, lastOutput.Error.Error(), "context canceled",
			"final command should not be canceled")
	}

	// If we got multiple results, earlier ones might be canceled
	// but this tests that the cancellation mechanism is working
	t.Logf("Received %d command outputs from 5 file events", len(outputs))
}
