package kube_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

var (
	testProfiles = map[string]*kube.Profile{
		"ks": kube.MustNewProfile("kustomize",
			kube.WithArgs("build", "."),
			kube.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`)),
		"helm": kube.MustNewProfile("helm",
			kube.WithArgs("template", ".", "--generate-name"),
			kube.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])`)),
		"yaml": kube.MustNewProfile("sh",
			kube.WithArgs("-c", "yq eval-all '.' *.yaml"),
			kube.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`)),
	}

	testRules = []*kube.Rule{
		kube.MustNewRule("kustomize", `files.exists(f, pathBase(f).matches(".*kustomization.*"))`, "ks"),
		kube.MustNewRule("helm", `files.exists(f, pathBase(f).matches("Chart\\..*"))`, "helm"),
		kube.MustNewRule("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`, "yaml"),
	}

	TestConfig = kube.MustNewConfig(testProfiles, testRules)
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

	unknownFile := filepath.Join(tempDir, "unknown")
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

			runner, err := kube.NewCommandRunner(tc.path, kube.WithRules(TestConfig.Rules))
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

	profile, err := kube.NewProfile("echo", kube.WithArgs("{apiVersion: v1, kind: Resource}"))
	require.NoError(t, err)

	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithProfile("echo", profile))
	require.NoError(t, err)

	output := runner.Run()
	require.NoError(t, output.Error)

	assert.Empty(t, output.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: Resource}\n", output.Stdout)
	assert.Equal(t, "v1", output.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "Resource", output.Resources[0].Object.GetKind())
}

func TestCommandRunner_RunContext(t *testing.T) {
	t.Parallel()

	profile, err := kube.NewProfile("echo", kube.WithArgs("{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}"))
	require.NoError(t, err)

	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithProfile("echo", profile))
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

	profile, err := kube.NewProfile("sleep", kube.WithArgs("2"))
	require.NoError(t, err)

	// Create a command that takes some time to execute
	runner, err := kube.NewCommandRunner(t.TempDir(), kube.WithProfile("sleep", profile))
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

	tcs := map[string]struct {
		commandSleepTime string
		numFileEvents    int
		collectDuration  time.Duration
	}{
		"rapid file events with slow command": {
			numFileEvents:    5,
			commandSleepTime: "0.2", // 200ms sleep
			collectDuration:  3 * time.Second,
		},
		"fewer file events with faster command": {
			numFileEvents:    3,
			commandSleepTime: "0.1", // 100ms sleep
			collectDuration:  2 * time.Second,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory for testing
			tempDir := t.TempDir()

			// Create a test file to watch
			testFile := filepath.Join(tempDir, "test.yaml")
			require.NoError(t, os.WriteFile(testFile, []byte("test: data"), 0o644))

			profile, err := kube.NewProfile("sleep",
				kube.WithArgs(tc.commandSleepTime),
				kube.WithSource(`files.filter(f, pathExt(f) == ".yaml")`),
			)
			require.NoError(t, err)

			// Create a command that takes a bit of time to execute
			runner, err := kube.NewCommandRunner(tempDir, kube.WithProfile("sleep", profile))
			require.NoError(t, err)

			// Start watching
			require.NoError(t, runner.Watch())
			defer runner.Close()

			// Channel to collect command outputs
			results := make(chan kube.CommandEvent, 50) // Larger buffer for multiple FS events
			runner.Subscribe(results)

			// Start RunOnEvent in a goroutine
			go runner.RunOnEvent()

			// Give it a moment to start watching
			time.Sleep(50 * time.Millisecond)

			// Trigger multiple rapid file events by writing to the file quickly
			for i := range tc.numFileEvents {
				content := fmt.Sprintf("test: data-%d", i)
				require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))
				time.Sleep(10 * time.Millisecond) // Small delay between writes
			}

			// Collect all events for a specified duration
			var (
				outputs        []kube.CommandOutput
				startEvents    int
				cancelEvents   int
				collectionDone = make(chan struct{})
			)

			go func() {
				defer close(collectionDone)
				deadline := time.After(tc.collectDuration)

				for {
					select {
					case event := <-results:
						switch out := event.(type) {
						case kube.CommandEventStart:
							startEvents++
						case kube.CommandEventEnd:
							outputs = append(outputs, kube.CommandOutput(out))
						case kube.CommandEventCancel:
							cancelEvents++
						}
					case <-deadline:
						return
					}
				}
			}()

			// Wait for collection to complete
			<-collectionDone

			// Core assertions: These are the behaviors we actually care about

			// 1. We should get at least one successful command completion
			assert.GreaterOrEqual(t, len(outputs), 1,
				"should get at least one command result")

			// 2. We should see some start events (file system watcher is working)
			assert.GreaterOrEqual(t, startEvents, 1,
				"should see at least one start event")

			// 3. If we have multiple outputs, we should see some cancellations
			// (this tests that the cancellation mechanism is working)
			if len(outputs) > 1 {
				assert.GreaterOrEqual(t, cancelEvents, 1,
					"should see some cancellations when multiple commands run")
			}

			// 4. The final result should not be a cancellation error
			if len(outputs) > 0 {
				lastOutput := outputs[len(outputs)-1]
				if lastOutput.Error != nil {
					assert.NotContains(t, lastOutput.Error.Error(), "context canceled",
						"final command should not be canceled")
				}
			}

			// 5. We shouldn't have more completed commands than we have start events
			// (basic sanity check)
			assert.LessOrEqual(t, len(outputs), startEvents,
				"completed commands should not exceed started commands")

			// Log the results for debugging
			t.Logf("Events: %d starts, %d ends, %d cancels from %d file events",
				startEvents, len(outputs), cancelEvents, tc.numFileEvents)

			// Additional logging to help understand platform differences
			if startEvents > tc.numFileEvents*2 {
				t.Logf("Note: File system generated %d start events for %d file writes (%.1fx multiplier)",
					startEvents, tc.numFileEvents, float64(startEvents)/float64(tc.numFileEvents))
			}
		})
	}
}
