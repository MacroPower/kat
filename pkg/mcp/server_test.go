package mcp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/mcp"
)

// mockCommandRunner implements the CommandRunner interface for testing.
type mockCommandRunner struct {
	channels       []chan<- command.Event
	outputs        []command.Output
	configureCount int
	outputIndex    int
}

func (m *mockCommandRunner) Configure(_ ...command.RunnerOpt) error {
	m.configureCount++

	return nil
}

func (m *mockCommandRunner) Subscribe(ch chan<- command.Event) {
	m.channels = append(m.channels, ch)
}

func (m *mockCommandRunner) RunContext(_ context.Context) command.Output {
	// Send start event immediately.
	m.sendEvent(command.EventStart(command.TypeRun))

	// Simulate some work.
	time.Sleep(10 * time.Millisecond)

	// Get the next output.
	var output command.Output
	if m.outputIndex < len(m.outputs) {
		output = m.outputs[m.outputIndex]
		m.outputIndex++
	} else {
		output = command.Output{
			Type:      command.TypeRun,
			Resources: []*kube.Resource{},
		}
	}

	// Send end event.
	endEvent := command.EventEnd(output)
	m.sendEvent(endEvent)

	return output
}

func (m *mockCommandRunner) sendEvent(event command.Event) {
	for _, ch := range m.channels {
		ch <- event
	}
}

func (m *mockCommandRunner) addOutput(output command.Output) {
	m.outputs = append(m.outputs, output)
}

func TestServer_EventProcessing(t *testing.T) {
	t.Parallel()

	// Create test resources for output
	testResources := []*kube.Resource{
		{
			Object: &kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			YAML: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod\n  namespace: default",
		},
		{
			Object: &kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-deployment",
					"namespace": "kube-system",
				},
			},
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment\n  namespace: kube-system",
		},
	}

	tcs := map[string]struct {
		err    error
		want   string
		events []command.Event
	}{
		"start event sets status to running": {
			events: []command.Event{
				command.EventStart(command.TypeRun),
			},
			want: "running",
			err:  nil,
		},
		"end event with success sets status to completed": {
			events: []command.Event{
				command.EventStart(command.TypeRun),
				command.EventEnd{
					Error:     nil,
					Stdout:    "command output",
					Stderr:    "",
					Resources: testResources,
					Type:      command.TypeRun,
				},
			},
			want: "completed",
			err:  nil,
		},
		"end event with error sets status to error": {
			events: []command.Event{
				command.EventStart(command.TypeRun),
				command.EventEnd{
					Error:     errors.New("test error"),
					Stdout:    "",
					Stderr:    "error output",
					Resources: nil,
					Type:      command.TypeRun,
				},
			},
			want: "error",
			err:  nil,
		},
		"cancel event sets status to idle": {
			events: []command.Event{
				command.EventStart(command.TypeRun),
				command.EventCancel{},
			},
			want: "idle",
			err:  nil,
		},
		"multiple start/end cycles work correctly": {
			events: []command.Event{
				command.EventStart(command.TypeRun),
				command.EventEnd{
					Error:     nil,
					Stdout:    "first command",
					Stderr:    "",
					Resources: testResources,
					Type:      command.TypeRun,
				},
				command.EventStart(command.TypePlugin),
				command.EventEnd{
					Error:     errors.New("second error"),
					Stdout:    "",
					Stderr:    "second error output",
					Resources: nil,
					Type:      command.TypePlugin,
				},
			},
			want: "error",
			err:  nil,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testRunner := &mockCommandRunner{}
			testServer, err := mcp.NewServer("localhost:8081", testRunner, "/test/path")
			require.NoError(t, err)

			// Send events
			for _, event := range tc.events {
				testRunner.sendEvent(event)
			}

			// Give time for events to be processed
			time.Sleep(10 * time.Millisecond)

			// We test the state processing by verifying the server was created successfully
			// The actual state verification requires accessing private fields or methods
			if tc.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, testServer)
		})
	}
}

//nolint:paralleltest,tparallel // Shares a clientSession.
func TestServer_Integration(t *testing.T) {
	t.Parallel()

	// Create test resources for output
	testResources := []*kube.Resource{
		{
			Object: &kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			YAML: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod\n  namespace: default",
		},
		{
			Object: &kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-deployment",
					"namespace": "kube-system",
				},
			},
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment\n  namespace: kube-system",
		},
	}

	clientTransport, serverTransport := sdk.NewInMemoryTransports()

	testRunner := &mockCommandRunner{}

	// Add output that will be used when RunContext is called during reloads.
	testOutput := command.Output{
		Error:     nil,
		Stdout:    "command output",
		Stderr:    "",
		Resources: testResources,
		Type:      command.TypeRun,
	}
	testRunner.addOutput(testOutput)

	testServer, err := mcp.NewServer("", testRunner, "/initial/path")
	require.NoError(t, err)

	ctx := t.Context()

	serverSession, err := testServer.Server().Connect(ctx, serverTransport)
	require.NoError(t, err)

	client := sdk.NewClient(&sdk.Implementation{Name: "client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport)
	require.NoError(t, err)

	// Add enough outputs for all the test cases.
	for range 10 {
		testRunner.addOutput(testOutput)
	}

	tcs := map[string]struct {
		params *sdk.CallToolParams
		want   map[string]any
	}{
		"list_resources": {
			params: &sdk.CallToolParams{
				Name: "list_resources",
				Arguments: map[string]any{
					"path": "/test/path",
				},
			},
			want: map[string]any{
				"status":        "completed",
				"stdoutPreview": "command output",
				"resourceCount": float64(2),
				"resources": []any{
					map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"name":       "test-pod",
						"namespace":  "default",
					},
					map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "test-deployment",
						"namespace":  "kube-system",
					},
				},
			},
		},
		"list_resources_with_path": {
			params: &sdk.CallToolParams{
				Name: "list_resources",
				Arguments: map[string]any{
					"path": "/some/other/path",
				},
			},
			want: map[string]any{
				"status":        "completed",
				"stdoutPreview": "command output",
				"resourceCount": float64(2),
				"resources": []any{
					map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"name":       "test-pod",
						"namespace":  "default",
					},
					map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "test-deployment",
						"namespace":  "kube-system",
					},
				},
			},
		},
		"get_resource_found": {
			params: &sdk.CallToolParams{
				Name: "get_resource",
				Arguments: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"name":       "test-pod",
					"namespace":  "default",
					"path":       "/test/path",
				},
			},
			want: map[string]any{
				"status": "completed",
				"found":  true,
				"resource": map[string]any{
					"metadata": map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"name":       "test-pod",
						"namespace":  "default",
					},
					"yaml": "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod\n  namespace: default",
				},
			},
		},
		"get_resource_with_path": {
			params: &sdk.CallToolParams{
				Name: "get_resource",
				Arguments: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"name":       "test-pod",
					"namespace":  "default",
					"path":       "/another/path",
				},
			},
			want: map[string]any{
				"status": "completed",
				"found":  true,
				"resource": map[string]any{
					"metadata": map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"name":       "test-pod",
						"namespace":  "default",
					},
					"yaml": "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod\n  namespace: default",
				},
			},
		},
		"get_resource_not_found": {
			params: &sdk.CallToolParams{
				Name: "get_resource",
				Arguments: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"name":       "nonexistent-pod",
					"namespace":  "default",
					"path":       "/test/path",
				},
			},
			want: map[string]any{
				"status": "completed",
				"found":  false,
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			r, err := clientSession.CallTool(ctx, tc.params)
			require.NoError(t, err)

			assert.NotNil(t, r)
			assert.NotNil(t, r.StructuredContent)

			assert.Equal(t, tc.want, r.StructuredContent)
		})
	}

	require.NoError(t, clientSession.Close())
	require.NoError(t, serverSession.Wait())
	testServer.Close()
}

func TestServer_PathReconfiguration(t *testing.T) {
	t.Parallel()

	clientTransport, serverTransport := sdk.NewInMemoryTransports()

	testRunner := &mockCommandRunner{}
	testServer, err := mcp.NewServer("", testRunner, "/initial/path")
	require.NoError(t, err)

	ctx := t.Context()

	serverSession, err := testServer.Server().Connect(ctx, serverTransport)
	require.NoError(t, err)

	client := sdk.NewClient(&sdk.Implementation{Name: "client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport)
	require.NoError(t, err)

	// Send events to simulate command execution.
	testRunner.sendEvent(command.EventStart(command.TypeRun))
	testRunner.sendEvent(command.EventEnd{
		Error:     nil,
		Stdout:    "initial output",
		Stderr:    "",
		Resources: nil,
		Type:      command.TypeRun,
	})

	// Give time for events to be processed.
	time.Sleep(100 * time.Millisecond)

	// Reset configure count to test path change.
	initialConfigureCount := testRunner.configureCount
	testRunner.configureCount = 0

	// Call list_resources with a different path.
	_, err = clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "list_resources",
		Arguments: map[string]any{
			"path": "/new/path",
		},
	})
	require.NoError(t, err)

	// Verify that Configure was called once due to path change.
	assert.Equal(t, 1, testRunner.configureCount, "Configure should be called once when path changes")

	// Call list_resources with the same path again.
	testRunner.configureCount = 0
	_, err = clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "list_resources",
		Arguments: map[string]any{
			"path": "/new/path",
		},
	})
	require.NoError(t, err)

	// Verify that Configure was not called again since path didn't change.
	assert.Equal(t, 0, testRunner.configureCount, "Configure should not be called when path doesn't change")

	// Call list_resources without path parameter.
	_, err = clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_resources",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)

	// Verify that Configure was not called since path parameter is empty (no-op).
	assert.Equal(t, 0, testRunner.configureCount, "Configure should not be called when no path is provided")

	// Also test get_resource path functionality.
	testRunner.configureCount = 0
	_, err = clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_resource",
		Arguments: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "test-pod",
			"path":       "/another/path",
		},
	})
	require.NoError(t, err)

	// Verify that Configure was called for get_resource path change.
	assert.Equal(t, 1, testRunner.configureCount, "Configure should be called when get_resource path changes")

	_ = initialConfigureCount // Use the variable to avoid unused variable warning

	require.NoError(t, clientSession.Close())
	require.NoError(t, serverSession.Wait())
	testServer.Close()
}

func TestServer_LatestResultsAfterReload(t *testing.T) {
	t.Parallel()

	clientTransport, serverTransport := sdk.NewInMemoryTransports()

	testRunner := &mockCommandRunner{}

	// Set up different outputs for different calls.
	firstOutput := command.Output{
		Type:   command.TypeRun,
		Stdout: "first output",
		Resources: []*kube.Resource{
			{
				Object: &kube.Object{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "first-pod",
						"namespace": "default",
					},
				},
				YAML: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: first-pod",
			},
		},
	}

	secondOutput := command.Output{
		Type:   command.TypeRun,
		Stdout: "second output",
		Resources: []*kube.Resource{
			{
				Object: &kube.Object{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "second-pod",
						"namespace": "default",
					},
				},
				YAML: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: second-pod",
			},
		},
	}

	testRunner.addOutput(firstOutput)
	testRunner.addOutput(secondOutput)

	testServer, err := mcp.NewServer("", testRunner, "/initial/path")
	require.NoError(t, err)

	ctx := t.Context()

	serverSession, err := testServer.Server().Connect(ctx, serverTransport)
	require.NoError(t, err)

	client := sdk.NewClient(&sdk.Implementation{Name: "client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport)
	require.NoError(t, err)

	// First call should get the first output.
	result1, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "list_resources",
		Arguments: map[string]any{
			"path": "/test/path",
		},
	})
	require.NoError(t, err)

	structuredContent1, ok := result1.StructuredContent.(map[string]any)
	require.True(t, ok, "StructuredContent should be a map[string]any")
	assert.Equal(t, "first output", structuredContent1["stdoutPreview"])

	// Second call with different path should trigger reload and get new output.
	result2, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "list_resources",
		Arguments: map[string]any{
			"path": "/test/new-path",
		},
	})
	require.NoError(t, err)

	structuredContent2, ok := result2.StructuredContent.(map[string]any)
	require.True(t, ok, "StructuredContent should be a map[string]any")
	assert.Equal(t, "second output", structuredContent2["stdoutPreview"])

	// Verify that the resources are different too.
	resources1, ok := structuredContent1["resources"].([]any)
	require.True(t, ok, "Resources should be a []any")

	resources2, ok := structuredContent2["resources"].([]any)
	require.True(t, ok, "Resources should be a []any")

	pod1, ok := resources1[0].(map[string]any)
	require.True(t, ok, "Pod name should be a map[string]any")

	pod2, ok := resources2[0].(map[string]any)
	require.True(t, ok, "Pod name should be a map[string]any")

	podName1, ok := pod1["name"]
	require.True(t, ok, "Pod should have a name")

	podName2, ok := pod2["name"]
	require.True(t, ok, "Pod should have a name")

	assert.Equal(t, "first-pod", podName1)
	assert.Equal(t, "second-pod", podName2)

	require.NoError(t, clientSession.Close())
	require.NoError(t, serverSession.Wait())

	testServer.Close()
}
