package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

func TestProfile_WithPostRenderHooks(t *testing.T) {
	t.Parallel()

	// Create a command with postRender hooks that use stdin
	cmd := &kube.Profile{
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

func TestProfile_FailingPostRenderHook(t *testing.T) {
	t.Parallel()

	// Create a command with a failing postRender hook
	cmd := &kube.Profile{
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

func TestProfile_EmptyHookCommand(t *testing.T) {
	t.Parallel()

	// Create a command with an empty hook command
	cmd := &kube.Profile{
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

func TestProfile_WithPreRenderHooks(t *testing.T) {
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
			cmd := &kube.Profile{
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

func TestProfile_PreRenderHookFailurePreventsMainCommand(t *testing.T) {
	t.Parallel()

	// Create a command with a failing preRender hook
	// The main command should not execute if preRender fails
	cmd := &kube.Profile{
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

func TestProfile_WithBothPreAndPostRenderHooks(t *testing.T) {
	t.Parallel()

	// Test that both preRender and postRender hooks execute successfully
	cmd := &kube.Profile{
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
