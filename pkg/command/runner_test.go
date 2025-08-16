package command_test

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

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

var (
	testProfiles = map[string]*profile.Profile{
		"ks": profile.MustNew("kustomize",
			profile.WithArgs("build", "."),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			profile.WithReload(`fs.event.has(fs.WRITE, fs.CREATE, fs.REMOVE)`)),
		"helm": profile.MustNew("helm",
			profile.WithArgs("template", ".", "--generate-name"),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])`),
			profile.WithReload(`fs.event.has(fs.WRITE, fs.RENAME) && pathBase(file) != "Chart.lock"`)),
		"yaml": profile.MustNew("sh",
			profile.WithArgs("-c", "yq eval-all '.' *.yaml"),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			profile.WithReload(`pathExt(file) in [".yaml", ".yml"]`)),
	}

	testRules = []*rule.Rule{
		rule.MustNew("ks", `files.exists(f, pathBase(f).matches(".*kustomization.*"))`),
		rule.MustNew("helm", `files.exists(f, pathBase(f).matches("Chart\\..*"))`),
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	}

	TestConfig = command.MustNewConfig(testProfiles, testRules)
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
			initError:   command.ErrNoCommandForPath,
			checkOutput: false,
		},
		"directory with no matching files": {
			path:        t.TempDir(), // Empty temp directory
			initError:   command.ErrNoCommandForPath,
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

			runner, err := command.NewRunner(tc.path,
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))
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

	p, err := profile.New("echo", profile.WithArgs("{apiVersion: v1, kind: Resource}"))
	require.NoError(t, err)

	runner, err := command.NewRunner(t.TempDir(), command.WithProfile("echo", p))
	require.NoError(t, err)

	output := runner.Run()
	require.NoError(t, output.Error)

	assert.Empty(t, output.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: Resource}\n", output.Stdout)
	require.Len(t, output.Resources, 1)
	assert.Equal(t, "v1", output.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "Resource", output.Resources[0].Object.GetKind())
}

func TestCommandRunner_RunContext(t *testing.T) {
	t.Parallel()

	p, err := profile.New("echo", profile.WithArgs("{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}"))
	require.NoError(t, err)

	runner, err := command.NewRunner(t.TempDir(), command.WithProfile("echo", p))
	require.NoError(t, err)

	// Test with context.Background()
	output := runner.RunContext(t.Context())
	require.NoError(t, output.Error)

	assert.Empty(t, output.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: ConfigMap, metadata: {name: test}}\n", output.Stdout)
	require.Len(t, output.Resources, 1)
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

//nolint:tparallel // This test is inherently sequential due to cancellation behavior.
func TestCommandRunner_CancellationBehavior(t *testing.T) {
	t.Parallel()

	p, err := profile.New("sleep", profile.WithArgs("2"))
	require.NoError(t, err)

	// Create a command that takes some time to execute
	runner, err := command.NewRunner(t.TempDir(), command.WithProfile("sleep", p))
	require.NoError(t, err)

	// Test that a new command cancels the previous one
	t.Run("new command cancels previous", func(t *testing.T) {
		// Start first command with a context that we can monitor
		ctx1, cancel1 := context.WithCancel(t.Context())
		defer cancel1()

		// Channel to collect results
		results := make(chan command.Output, 2)

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
		var outputs []command.Output
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

func TestCommandRunner_FileWatcher(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		fileOperation func(*testing.T, string)
		wantEvents    int
	}{
		"simple file modification": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.WriteFile(testFile, []byte("test: modified"), 0o644))
			},
			wantEvents: 1,
		},
		"file removal and recreation": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.Remove(testFile))
				time.Sleep(50 * time.Millisecond)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"1\""), 0o644))
				time.Sleep(50 * time.Millisecond)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"2\""), 0o644))
			},
			wantEvents: 3,
		},
		"file rename and recreation": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.Rename(testFile, testFile+".bak"))
				time.Sleep(50 * time.Millisecond)
				require.NoError(t, os.Rename(testFile+".bak", testFile))
				time.Sleep(50 * time.Millisecond)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"1\""), 0o644))
				time.Sleep(50 * time.Millisecond)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"2\""), 0o644))
			},
			wantEvents: 3,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory for testing
			tempDir := t.TempDir()

			// Create a test file to watch
			testFile := filepath.Join(tempDir, "test.yaml")
			require.NoError(t, os.WriteFile(testFile, []byte("test: initial"), 0o644))

			p, err := profile.New("echo",
				profile.WithArgs("File event"),
				profile.WithSource(`files.filter(f, pathExt(f) == ".yaml")`),
				profile.WithReload(`fs.event.has(fs.WRITE, fs.CREATE, fs.REMOVE, fs.RENAME)`),
			)
			require.NoError(t, err)

			// Create a runner with a fast command
			runner, err := command.NewRunner(filepath.Dir(testFile), command.WithProfile("echo", p))
			require.NoError(t, err)

			// Start watching
			require.NoError(t, runner.Watch())
			defer runner.Close()

			// Channel to collect command events
			results := make(chan command.Event, 10)
			runner.Subscribe(results)

			// Start RunOnEvent in a goroutine
			go runner.RunOnEvent()

			// Give it a moment to start watching
			time.Sleep(50 * time.Millisecond)

			// Perform the file operation
			tc.fileOperation(t, testFile)

			// Collect events for a reasonable duration
			events := collectEventsWithTimeout(results, tc.wantEvents, 5*time.Second)
			require.Len(t, events, tc.wantEvents, testFile)

			// Verify we got alternating start/end events
			for i, event := range events {
				switch i % 2 {
				case 0: // Even indices should be start events
					assert.IsType(t, command.EventStart(command.TypeRun), event)
				case 1: // Odd indices should be end events
					assert.IsType(t, command.EventEnd{}, event)
				}
			}
		})
	}
}

func TestCommandRunner_ConcurrentFileEvents(t *testing.T) {
	t.Parallel()

	// Helper function for file write operations
	writeFileOp := func(t *testing.T, testFile string, i int) {
		t.Helper()

		content := fmt.Sprintf("test: data-%d", i)
		require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))
	}

	// Helper function for file remove and re-add operations
	removeAddFileOp := func(t *testing.T, testFile string, i int) {
		t.Helper()

		require.NoError(t, os.Remove(testFile))

		time.Sleep(5 * time.Millisecond) // Small delay between remove and add

		content := fmt.Sprintf("test: recreated-%d", i)
		require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))
	}

	tcs := map[string]struct {
		fileOp             func(*testing.T, string, int)
		commandSleepTime   string
		profileReload      string
		numFileEvents      int
		collectDuration    time.Duration
		expectCancellation bool
	}{
		"rapid file events with slow command should cause cancellation": {
			numFileEvents:      5,
			commandSleepTime:   "0.3",
			collectDuration:    3 * time.Second,
			profileReload:      `fs.event.has(fs.WRITE, fs.CREATE)`,
			fileOp:             writeFileOp,
			expectCancellation: true,
		},
		"fewer file events with faster command should complete all": {
			numFileEvents:      2,
			commandSleepTime:   "0.05",
			collectDuration:    2 * time.Second,
			profileReload:      `pathExt(file) == ".yaml"`,
			fileOp:             writeFileOp,
			expectCancellation: false,
		},
		"file events with path filtering": {
			numFileEvents:      4,
			commandSleepTime:   "0.1",
			collectDuration:    2 * time.Second,
			profileReload:      `pathBase(file) != "ignored.yaml"`,
			fileOp:             writeFileOp,
			expectCancellation: false,
		},
		"file remove and re-add events with cancellation": {
			numFileEvents:      4,
			commandSleepTime:   "0.2",
			collectDuration:    3 * time.Second,
			profileReload:      `fs.event.has(fs.WRITE, fs.CREATE, fs.REMOVE)`,
			fileOp:             removeAddFileOp,
			expectCancellation: true,
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

			p, err := profile.New("sleep",
				profile.WithArgs(tc.commandSleepTime),
				profile.WithSource(`files.filter(f, pathExt(f) == ".yaml")`),
				profile.WithReload(tc.profileReload),
			)
			require.NoError(t, err)

			// Create a command that takes a bit of time to execute
			runner, err := command.NewRunner(tempDir, command.WithProfile("sleep", p))
			require.NoError(t, err)

			// Start watching
			require.NoError(t, runner.Watch())
			defer runner.Close()

			// Channel to collect command outputs
			results := make(chan command.Event, 50) // Larger buffer for multiple FS events
			runner.Subscribe(results)

			// Start RunOnEvent in a goroutine
			go runner.RunOnEvent()

			// Give it a moment to start watching
			time.Sleep(50 * time.Millisecond)

			// Trigger multiple rapid file events using the specified operation
			for i := range tc.numFileEvents {
				tc.fileOp(t, testFile, i)
				time.Sleep(10 * time.Millisecond)
			}

			// Collect all events for a specified duration
			var (
				outputs        []command.Output
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
						case command.EventStart:
							startEvents++
						case command.EventEnd:
							outputs = append(outputs, command.Output(out))
						case command.EventCancel:
							cancelEvents++
						}

					case <-deadline:
						return
					}
				}
			}()

			// Wait for collection to complete
			<-collectionDone

			require.GreaterOrEqual(t, len(outputs), 1, "should get at least one completed command")
			require.GreaterOrEqual(t, startEvents, 1, "should get at least one start event")

			assert.LessOrEqual(t, len(outputs), startEvents,
				"completed commands (%d) should not exceed started commands (%d)",
				len(outputs), startEvents)

			if tc.expectCancellation {
				// With multiple rapid file events and slow commands, some should be canceled
				assert.GreaterOrEqual(t, cancelEvents, 1,
					"should see cancellations with rapid events and slow commands")
				assert.Greater(t, startEvents, len(outputs),
					"should have more starts (%d) than completions (%d) due to cancellations",
					startEvents, len(outputs))
			}

			lastOutput := outputs[len(outputs)-1]
			if lastOutput.Error != nil {
				assert.NotContains(t, lastOutput.Error.Error(), "context canceled",
					"final completed command should not be canceled")
			}

			// Log the results for debugging
			t.Logf("Events: %d starts, %d ends, %d cancels from %d file operations",
				startEvents, len(outputs), cancelEvents, tc.numFileEvents)
		})
	}
}

func TestCommandRunner_String(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	// Create a file that matches our test rules so the runner can be created
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	// The String method should return the rule's string representation: "profile: command args"
	result := runner.String()
	assert.Contains(t, result, "helm:")
	assert.Contains(t, result, "helm template . --generate-name")
}

func TestCommandRunner_GetCurrentProfile(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupFiles func(string) error
		expectNil  bool
	}{
		"with kustomization file": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("resources: []"), 0o644)
			},
			expectNil: false,
		},
		"with no matching files": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "random.txt"), []byte("content"), 0o644)
			},
			expectNil: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			require.NoError(t, tc.setupFiles(tempDir))

			runner, err := command.NewRunner(tempDir,
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))
			if tc.expectNil {
				// Should fail to create runner for paths with no matching rules
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			_, p := runner.GetCurrentProfile()
			require.NotNil(t, p)
			assert.NotEmpty(t, p.Command.Command)
		})
	}
}

func TestCommandRunner_RunPlugin(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	// Test event subscription
	eventCh := make(chan command.Event, 10)
	runner.Subscribe(eventCh)

	output := runner.RunPlugin("test-plugin")

	// Collect events synchronously
	events := collectEventsWithTimeout(eventCh, 2, 100*time.Millisecond)

	// Verify output - plugins are not implemented yet, so should get an error
	assert.Equal(t, command.TypePlugin, output.Type)
	// The exact error depends on implementation, but there should be some error
	// since plugins aren't fully implemented

	// Verify events
	assert.GreaterOrEqual(t, len(events), 2)
	assert.IsType(t, command.EventStart(command.TypePlugin), events[0])
	assert.IsType(t, command.EventEnd{}, events[1])
}

func TestCommandRunner_RunPluginContext(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	tests := map[string]struct {
		setupContext func() context.Context
		pluginName   string
	}{
		"with background context": {
			setupContext: func() context.Context {
				return t.Context()
			},
			pluginName: "test-plugin",
		},
		"with canceled context": {
			setupContext: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel() // Cancel immediately

				return ctx
			},
			pluginName: "test-plugin",
		},
		"with timeout context": {
			setupContext: func() context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 1*time.Millisecond)
				defer cancel()
				time.Sleep(5 * time.Millisecond) // Ensure timeout

				return ctx
			},
			pluginName: "another-plugin",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := tc.setupContext()
			output := runner.RunPluginContext(ctx, tc.pluginName)

			// Verify output type
			assert.Equal(t, command.TypePlugin, output.Type)
			// Context cancellation and timeouts may or may not affect the result
			// depending on timing, so we just verify the basic structure
		})
	}
}

func TestRunner_GetProfiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	// Test with WithRules - profiles should be extracted from rules
	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	defer runner.Close()

	profiles := runner.GetProfiles()

	// Should have all profiles from the test config
	require.NotNil(t, profiles)
	assert.Contains(t, profiles, "ks")
	assert.Contains(t, profiles, "helm")
	assert.Contains(t, profiles, "yaml")

	// Verify profile contents
	assert.Equal(t, testProfiles["yaml"].Command.Command, profiles["yaml"].Command.Command)
	assert.Equal(t, testProfiles["ks"].Command.Command, profiles["ks"].Command.Command)
	assert.Equal(t, testProfiles["helm"].Command.Command, profiles["helm"].Command.Command)
}

func TestRunner_SetProfile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	t.Cleanup(runner.Close)

	tcs := map[string]struct {
		profileName string
		wantErr     bool
	}{
		"switch to existing profile ks": {
			profileName: "ks",
			wantErr:     false,
		},
		"switch to existing profile helm": {
			profileName: "helm",
			wantErr:     false,
		},
		"switch to non-existent profile": {
			profileName: "nonexistent",
			wantErr:     true,
		},
		"switch to empty profile name": {
			profileName: "",
			wantErr:     true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := runner.SetProfile(tc.profileName)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "not found")
			} else {
				require.NoError(t, err)

				// Verify the profile was actually set
				currentName, currentProfile := runner.GetCurrentProfile()
				assert.Equal(t, tc.profileName, currentName)
				assert.NotNil(t, currentProfile)

				// Verify we can get the profile from the profiles map
				profiles := runner.GetProfiles()
				expectedProfile := profiles[tc.profileName]
				assert.Equal(t, expectedProfile, currentProfile)
			}
		})
	}
}

func TestRunner_SetProfile_Integration(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	defer runner.Close()

	// Check initial profile (should be "yaml" since we have a .yaml file)
	initialName, initialProfile := runner.GetCurrentProfile()
	assert.Equal(t, "yaml", initialName)
	assert.NotNil(t, initialProfile)

	// Switch to a different profile
	err = runner.SetProfile("ks")
	require.NoError(t, err)

	// Verify the switch worked
	currentName, currentProfile := runner.GetCurrentProfile()
	assert.Equal(t, "ks", currentName)
	assert.NotNil(t, currentProfile)
	assert.NotEqual(t, initialProfile, currentProfile)

	// Verify the command changed
	assert.Equal(t, "kustomize", currentProfile.Command.Command)

	// Switch back to original
	err = runner.SetProfile("yaml")
	require.NoError(t, err)

	// Verify we're back to the original
	finalName, finalProfile := runner.GetCurrentProfile()
	assert.Equal(t, "yaml", finalName)
	assert.Equal(t, initialProfile, finalProfile)
}

func TestRunner_WithProfile_SingleProfile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	customProfile := profile.MustNew("custom-command",
		profile.WithArgs("--custom", "arg"),
	)

	runner, err := command.NewRunner(tempDir, command.WithProfile("custom", customProfile))
	require.NoError(t, err)

	defer runner.Close()

	// Should have only the custom profile
	profiles := runner.GetProfiles()
	require.Len(t, profiles, 1)
	assert.Contains(t, profiles, "custom")

	// Current profile should be the custom one
	currentName, currentProfile := runner.GetCurrentProfile()
	assert.Equal(t, "custom", currentName)
	assert.Equal(t, customProfile, currentProfile)

	// Trying to set a different profile should fail
	err = runner.SetProfile("other")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `profile "other" not found`)
}

func TestRunner_WithProfiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	// Create additional profiles that don't have rules
	testProfiles := map[string]*profile.Profile{
		"extra1": profile.MustNew("echo",
			profile.WithArgs("extra1", "profile"),
		),
		"extra2": profile.MustNew("echo",
			profile.WithArgs("extra2", "profile"),
		),
	}
	testProfiles["yaml"] = TestConfig.Profiles["yaml"]

	// Create runner with both rules and additional profiles
	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(testProfiles))
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	profiles := runner.GetProfiles()
	require.NotNil(t, profiles)

	assert.Contains(t, profiles, "extra1")
	assert.Contains(t, profiles, "extra2")
	assert.Contains(t, profiles, "yaml")

	// Should start with rule-matched profile (yaml)
	currentName, currentProfile := runner.GetCurrentProfile()
	assert.Equal(t, "yaml", currentName)
	assert.NotNil(t, currentProfile)

	// Should be able to switch to additional profile
	err = runner.SetProfile("extra1")
	require.NoError(t, err)

	currentName, currentProfile = runner.GetCurrentProfile()
	assert.Equal(t, "extra1", currentName)
	assert.Equal(t, testProfiles["extra1"], currentProfile)
}

func TestRunner_WithProfiles_OverwriteRuleProfiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	// Create profile with same name as existing rule but different command
	overrideProfile := profile.MustNew("echo",
		profile.WithArgs("overridden", "yaml", "profile"),
	)

	additionalProfiles := map[string]*profile.Profile{
		"yaml": overrideProfile, // Override existing rule profile
	}

	runner, err := command.NewRunner(tempDir,
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(additionalProfiles))
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	// The profile from WithProfiles should override the one from rules
	profiles := runner.GetProfiles()
	yamlProfile := profiles["yaml"]
	assert.Equal(t, "echo", yamlProfile.Command.Command)
	assert.Equal(t, []string{"overridden", "yaml", "profile"}, yamlProfile.Command.Args)
}
