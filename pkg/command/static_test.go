package command_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/command"
)

func TestNewStatic(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input        string
		errorMsg     string
		numResources int
		expectError  bool
	}{
		"valid yaml input": {
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: dGVzdA==`,
			expectError:  false,
			numResources: 2,
		},
		"single resource": {
			input: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test
    image: nginx`,
			expectError:  false,
			numResources: 1,
		},
		"empty input": {
			input:       "",
			expectError: true,
			errorMsg:    "input cannot be empty",
		},
		"invalid yaml": {
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  invalid-yaml: [unclosed`,
			expectError: true,
			errorMsg:    "split yaml",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			static, err := command.NewStatic(tc.input)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
				assert.Nil(t, static)
			} else {
				require.NoError(t, err)
				require.NotNil(t, static)
				assert.Len(t, static.Resources, tc.numResources)
			}
		})
	}
}

func TestStatic_String(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	assert.Equal(t, "static", static.String())
}

func TestStatic_GetCurrentProfile(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	profile := static.GetCurrentProfile()
	require.NotNil(t, profile)
	// Static resources return an empty profile
	assert.Empty(t, profile.Command.Command)
}

func TestStatic_Run(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input        string
		errorMsg     string
		numResources int
		expectError  bool
		checkEvents  bool
	}{
		"successful run with resources": {
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`,
			expectError:  false,
			numResources: 1,
			checkEvents:  true,
		},
		"run with no resources": {
			input:       ``,
			expectError: true,
			errorMsg:    "no resources available",
			checkEvents: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				static *command.Static
				err    error
			)

			if tc.input == "" {
				// For the "no resources" case, we need to create a static instance
				// with empty resources manually
				static = &command.Static{Resources: nil}
			} else {
				static, err = command.NewStatic(tc.input)
				require.NoError(t, err)
			}

			// Test event subscription if needed
			var (
				events []command.Event
				output command.Output
			)

			if tc.checkEvents {
				events, output = runStaticWithEvents(t, static)
			} else {
				output = static.Run()
			}

			// Verify output
			verifyStaticOutput(t, output, tc.expectError, tc.errorMsg, tc.numResources)

			// Verify events if we collected them
			if tc.checkEvents {
				verifyStaticEvents(t, events, output)
			}
		})
	}
}

func TestStatic_RunPlugin(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	// Test event subscription
	eventCh := make(chan command.Event, 10)
	static.Subscribe(eventCh)

	output := static.RunPlugin("test-plugin")

	// Collect events synchronously
	events := collectEventsWithTimeout(eventCh, 2, 100*time.Millisecond)

	// Verify output
	assert.Equal(t, command.TypePlugin, output.Type)
	require.Error(t, output.Error)
	assert.Contains(t, output.Error.Error(), "plugins not supported in static resource mode")

	// Verify events
	assert.Len(t, events, 2)
	assert.IsType(t, command.EventStart(command.TypePlugin), events[0])
	assert.IsType(t, command.EventEnd{}, events[1])

	endEvent, ok := events[1].(command.EventEnd)
	require.True(t, ok, "expected second event to be EventEnd")
	assert.Equal(t, output, command.Output(endEvent))
}

func TestStatic_RunOnEvent(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	// RunOnEvent should do nothing for static resources
	// This test just ensures it doesn't panic
	static.RunOnEvent()
}

func TestStatic_Close(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	// Close should do nothing for static resources
	// This test just ensures it doesn't panic
	static.Close()
}

func TestStatic_Subscribe(t *testing.T) {
	t.Parallel()

	static, err := command.NewStatic(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)
	require.NoError(t, err)

	// Test multiple subscriptions
	ch1 := make(chan command.Event, 5)
	ch2 := make(chan command.Event, 5)

	static.Subscribe(ch1)
	static.Subscribe(ch2)

	// Run a command that generates events
	output := static.Run()
	require.NoError(t, output.Error)

	// Both channels should receive the events
	for _, ch := range []chan command.Event{ch1, ch2} {
		var events []command.Event

		// Collect events with timeout
		for i := range 2 {
			select {
			case event := <-ch:
				events = append(events, event)
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("timeout waiting for event %d", i+1)
			}
		}

		assert.Len(t, events, 2)
		assert.IsType(t, command.EventStart(command.TypeRun), events[0])
		assert.IsType(t, command.EventEnd{}, events[1])
	}
}

func runStaticWithEvents(t *testing.T, static *command.Static) ([]command.Event, command.Output) {
	t.Helper()

	eventCh := make(chan command.Event, 10)
	static.Subscribe(eventCh)

	// Run the command
	output := static.Run()

	// Collect events from the channel synchronously
	events := collectEventsWithTimeout(eventCh, 2, 100*time.Millisecond)

	return events, output
}

func verifyStaticOutput(t *testing.T, output command.Output, expectError bool, errorMsg string, numResources int) {
	t.Helper()

	assert.Equal(t, command.TypeRun, output.Type)
	if expectError {
		require.Error(t, output.Error)
		assert.Contains(t, output.Error.Error(), errorMsg)
	} else {
		require.NoError(t, output.Error)
		assert.Len(t, output.Resources, numResources)
	}
}

func verifyStaticEvents(t *testing.T, events []command.Event, output command.Output) {
	t.Helper()

	assert.Len(t, events, 2)
	assert.IsType(t, command.EventStart(command.TypeRun), events[0])
	assert.IsType(t, command.EventEnd{}, events[1])

	endEvent, ok := events[1].(command.EventEnd)
	require.True(t, ok, "expected second event to be EventEnd")
	assert.Equal(t, output, command.Output(endEvent))
}
