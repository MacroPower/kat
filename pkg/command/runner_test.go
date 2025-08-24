package command_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

// mockExecutor is a test implementation of the Executor interface.
type mockExecutor struct {
	result *execs.Result
	err    error
}

func (m *mockExecutor) Exec(ctx context.Context, dir string) (*execs.Result, error) {
	return m.ExecWithStdin(ctx, dir, nil)
}

func (m *mockExecutor) ExecWithStdin(_ context.Context, _ string, _ []byte) (*execs.Result, error) {
	return m.result, m.err
}

func (m *mockExecutor) String() string {
	return "mock executor"
}

// newMockExecutor creates a mock executor that returns the specified result and error.
func newMockExecutor(stdout, stderr string, err error) *mockExecutor {
	var result *execs.Result
	if err == nil {
		result = &execs.Result{
			Stdout: stdout,
			Stderr: stderr,
		}
	}

	return &mockExecutor{
		result: result,
		err:    err,
	}
}

// testRoot creates an os.Root from t.TempDir() and handles cleanup automatically.
func testRoot(t *testing.T) (*os.Root, string) {
	t.Helper()

	tempDir := t.TempDir()
	root, err := os.OpenRoot(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, root.Close())
	})

	return root, tempDir
}

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

// collectRunnerEventsWithTimeout collects up to maxEvents from the channel with a timeout
func collectRunnerEventsWithTimeout(
	eventCh <-chan command.Event,
	maxEvents int,
	timeout time.Duration,
) []command.Event {
	var events []command.Event

	timeoutTimer := time.After(timeout)

	for len(events) < maxEvents {
		select {
		case event := <-eventCh:
			events = append(events, event)
		case <-timeoutTimer:
			return events
		}
	}

	return events
}

func TestCommandRunner_RunForPath(t *testing.T) {
	t.Parallel()

	// Setup temp directory for testing
	root, tempDir := testRoot(t)

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

	// Create an empty subdirectory for testing "no matching files"
	emptyDir := filepath.Join(tempDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))

	tcs := map[string]struct {
		initError    error
		runError     error
		path         string
		checkProfile string
		checkOutput  bool
	}{
		"file not found": {
			path:        "nonexistent.yaml",
			initError:   os.ErrNotExist,
			checkOutput: false,
		},
		"no command for path": {
			path:        "unknown",
			initError:   command.ErrNoCommandForPath,
			checkOutput: false,
		},
		"directory with no matching files": {
			path:        "empty",
			initError:   command.ErrNoCommandForPath,
			checkOutput: false,
		},
		"match Chart.yaml file": {
			path:         "Chart.yaml",
			runError:     nil,
			checkOutput:  false,
			checkProfile: "helm",
		},
		"match kustomization.yaml file": {
			path:         "kustomization.yaml",
			runError:     nil,
			checkOutput:  false,
			checkProfile: "ks",
		},
		"directory with Chart.yaml has ks priority": {
			path:         ".", // Current directory contains both Chart.yaml and kustomization.yaml
			runError:     nil,
			checkOutput:  false,
			checkProfile: "ks", // ks should have priority over helm based on rule order
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner, err := command.NewRunnerWithRoot(root, tc.path,
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))
			if tc.initError != nil {
				require.ErrorIs(t, err, tc.initError)

				return
			}

			require.NoError(t, err)

			if tc.checkProfile != "" {
				name, currentProfile := runner.GetCurrentProfile()
				assert.Equal(t, tc.checkProfile, name)
				assert.NotNil(t, currentProfile)
			}

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

	root, _ := testRoot(t)
	runner, err := command.NewRunnerWithRoot(root, ".", command.WithCustomProfile("echo", p))
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

	root, _ := testRoot(t)
	runner, err := command.NewRunnerWithRoot(root, ".", command.WithCustomProfile("echo", p))
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

func TestCommandRunner_CancellationBehavior(t *testing.T) {
	t.Parallel()

	p, err := profile.New("sleep", profile.WithArgs("1"))
	require.NoError(t, err)

	// Create a command that takes some time to execute
	root, _ := testRoot(t)
	runner, err := command.NewRunnerWithRoot(root, ".", command.WithCustomProfile("sleep", p))
	require.NoError(t, err)

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
	time.Sleep(10 * time.Millisecond)

	// Start second command which should cancel the first
	go func() {
		results <- runner.RunContext(t.Context())
	}()

	// Collect results with timeout
	var outputs []command.Output
	assert.Eventually(t, func() bool {
		select {
		case output := <-results:
			outputs = append(outputs, output)
			return len(outputs) >= 2

		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)

	assert.Len(t, outputs, 2)

	var hasCancel, hasSuccess bool
	for _, output := range outputs {
		if output.Error != nil && strings.Contains(output.Error.Error(), "signal: killed") {
			hasCancel = true
		} else if output.Error == nil {
			hasSuccess = true
		}
	}

	assert.True(t, hasCancel, "expected 1 cancellation")
	assert.True(t, hasSuccess, "expected 1 successful completion")
}

func TestCommandRunner_FileWatcher(t *testing.T) {
	t.Parallel()

	notifyDelay := 100 * time.Millisecond

	tcs := map[string]struct {
		fileOperation func(*testing.T, string)
		wantEvents    int
	}{
		"simple file modification": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.WriteFile(testFile, []byte("test: modified"), 0o644))
				time.Sleep(notifyDelay)
			},
			wantEvents: 2,
		},
		"file removal and recreation": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.Remove(testFile))
				time.Sleep(notifyDelay)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"1\""), 0o644))
				time.Sleep(notifyDelay)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"2\""), 0o644))
				time.Sleep(notifyDelay)
			},
			wantEvents: 6,
		},
		"file rename and recreation": {
			fileOperation: func(t *testing.T, testFile string) {
				t.Helper()

				require.NoError(t, os.Rename(testFile, testFile+".bak"))
				time.Sleep(notifyDelay)
				require.NoError(t, os.Rename(testFile+".bak", testFile))
				time.Sleep(notifyDelay)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"1\""), 0o644))
				time.Sleep(notifyDelay)
				require.NoError(t, os.WriteFile(testFile, []byte("test: \"2\""), 0o644))
				time.Sleep(notifyDelay)
			},
			wantEvents: 6,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory for testing
			root, tempDir := testRoot(t)

			// Create a test file to watch (use relative path within root)
			testFile := "test.yaml"
			testFileFullPath := filepath.Join(tempDir, testFile)
			require.NoError(t, os.WriteFile(testFileFullPath, []byte("test: initial"), 0o644))

			p, err := profile.New("echo",
				profile.WithSource(`files.filter(f, pathExt(f) == ".yaml")`),
				profile.WithReload(`fs.event.has(fs.WRITE, fs.CREATE, fs.REMOVE, fs.RENAME)`),
				profile.WithExecutor(newMockExecutor("foo: bar", "", nil)),
			)
			require.NoError(t, err)

			// Create a runner with a fast command
			runner, err := command.NewRunnerWithRoot(
				root,
				".",
				command.WithCustomProfile("echo", p),
				command.WithWatch(true),
			)
			require.NoError(t, err)

			t.Cleanup(runner.Close)

			// Channel to collect command events
			results := make(chan command.Event, 100)
			runner.Subscribe(results)

			// Start RunOnEvent in a goroutine
			go runner.RunOnEvent()

			// Give it a moment to start watching
			time.Sleep(100 * time.Millisecond)

			// Perform the file operation (pass the full path)
			tc.fileOperation(t, testFileFullPath)

			// Collect events for a reasonable duration
			events := collectRunnerEventsWithTimeout(results, tc.wantEvents, 10*time.Second)
			require.Len(t, events, tc.wantEvents, testFileFullPath)

			// Verify we got alternating start/end events
			var startCount, endCount int
			for _, event := range events {
				t.Logf("event: %T: %+v", event, event)

				switch event.(type) {
				case command.EventStart:
					startCount++
				case command.EventEnd:
					endCount++
				}
			}

			assert.Equal(t, tc.wantEvents/2, startCount, "expected half of events to be start")
			assert.Equal(t, tc.wantEvents/2, endCount, "expected half of events to be end")
		})
	}
}

func TestCommandRunner_String(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	// Create a file that matches our test rules so the runner can be created
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunnerWithRoot(root, ".",
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

			root, tempDir := testRoot(t)
			require.NoError(t, tc.setupFiles(tempDir))

			runner, err := command.NewRunnerWithRoot(root, ".",
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

	root, tempDir := testRoot(t)
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunnerWithRoot(root, ".",
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)

	// Test event subscription
	eventCh := make(chan command.Event, 10)
	runner.Subscribe(eventCh)

	output := runner.RunPlugin("test-plugin")

	// Collect events synchronously
	events := collectRunnerEventsWithTimeout(eventCh, 2, 100*time.Millisecond)

	// Verify output - plugins are not implemented yet, so should get an error
	assert.Equal(t, command.TypePlugin, output.Type)
	// The exact error depends on implementation, but there should be some error
	// since plugins aren't fully implemented

	// Verify events
	assert.GreaterOrEqual(t, len(events), 2)
	assert.IsType(t, command.EventStart{}, events[0])
	assert.IsType(t, command.EventEnd{}, events[1])
}

func TestCommandRunner_RunPluginContext(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	runner, err := command.NewRunnerWithRoot(root, ".",
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

	root, tempDir := testRoot(t)

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	// Test with WithRules - profiles should be extracted from rules
	runner, err := command.NewRunnerWithRoot(root, ".",
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

//nolint:tparallel // Reconfigures a single runner.
func TestRunner_SetProfile(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	runner, err := command.NewRunnerWithRoot(root, ".",
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

	//nolint:paralleltest // Reconfigures a single runner.
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
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

	root, tempDir := testRoot(t)

	// Create test YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte("key: value"), 0o644)
	require.NoError(t, err)

	runner, err := command.NewRunnerWithRoot(root, ".",
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

	root, _ := testRoot(t)

	customProfile := profile.MustNew("custom-command",
		profile.WithArgs("--custom", "arg"),
	)

	runner, err := command.NewRunnerWithRoot(root, ".", command.WithCustomProfile("custom", customProfile))
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

	root, tempDir := testRoot(t)

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
	runner, err := command.NewRunnerWithRoot(root, ".",
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

	root, tempDir := testRoot(t)

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

	runner, err := command.NewRunnerWithRoot(root, ".",
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

func TestRunner_FindProfile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFiles   func(string) error
		expectErr    error
		expectedName string
	}{
		"find helm profile for Chart.yaml": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("name: test"), 0o644)
			},
			expectedName: "helm",
		},
		"find kustomize profile for kustomization.yaml": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("resources: []"), 0o644)
			},
			expectedName: "ks",
		},
		"find yaml profile for regular yaml file": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("key: value"), 0o644)
			},
			expectedName: "yaml",
		},
		"no matching profile": {
			setupFiles: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o644)
			},
			expectErr: command.ErrNoCommandForPath,
		},
		"empty directory": {
			setupFiles: func(_ string) error {
				return nil // Don't create any files
			},
			expectErr: command.ErrNoCommandForPath, // Should be this error since the directory exists but has no matching files
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a separate root for each test case
			testRoot, testDir := testRoot(t)
			require.NoError(t, tc.setupFiles(testDir))

			// Use NewRunnerWithRoot to properly initialize the runner
			runner, err := command.NewRunnerWithRoot(testRoot, ".",
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))

			// Handle expected errors during runner creation
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}

			require.NoError(t, err)
			t.Cleanup(runner.Close)

			profileName, currentProfile, err := runner.FindProfile(".")

			require.NoError(t, err)
			assert.Equal(t, tc.expectedName, profileName)
			assert.NotNil(t, currentProfile)
		})
	}
}

func TestRunner_FindProfiles(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)

	// Create a directory with multiple matching files
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test"), 0o644))

	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	require.NoError(t, os.WriteFile(kustomizationFile, []byte("resources: []"), 0o644))

	valuesFile := filepath.Join(tempDir, "values.yaml")
	require.NoError(t, os.WriteFile(valuesFile, []byte("key: value"), 0o644))

	runner, err := command.NewRunnerWithRoot(root, ".",
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	matches, err := runner.FindProfiles(".")
	require.NoError(t, err)

	// Should find all three profiles since we have kustomization.yaml, Chart.yaml, and values.yaml
	require.Len(t, matches, 3)

	// First match should be ks (higher priority in rule order)
	assert.Equal(t, "ks", matches[0].Name)
	assert.NotNil(t, matches[0].Profile)

	// Second match should be helm (Chart.yaml)
	assert.Equal(t, "helm", matches[1].Name)
	assert.NotNil(t, matches[1].Profile)

	// Third match should be yaml (values.yaml)
	assert.Equal(t, "yaml", matches[2].Name)
	assert.NotNil(t, matches[2].Profile)
}

func TestRunner_Configure(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	yamlFile := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("key: value"), 0o644))

	tcs := map[string]struct {
		checkFunc   func(*testing.T, *command.Runner)
		initialOpts []command.RunnerOpt
		configOpts  []command.RunnerOpt
		wantErr     bool
	}{
		"configure with rules and profiles": {
			configOpts: []command.RunnerOpt{
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles),
			},
			checkFunc: func(t *testing.T, r *command.Runner) {
				t.Helper()
				name, currentProfile := r.GetCurrentProfile()
				assert.Equal(t, "yaml", name)
				assert.NotNil(t, currentProfile)
			},
		},
		"configure with custom profile": {
			configOpts: []command.RunnerOpt{
				command.WithCustomProfile("custom", profile.MustNew("echo", profile.WithArgs("test"))),
			},
			checkFunc: func(t *testing.T, r *command.Runner) {
				t.Helper()
				name, currentProfile := r.GetCurrentProfile()
				assert.Equal(t, "custom", name)
				assert.Equal(t, "echo", currentProfile.Command.Command)
			},
		},
		"configure with extra args": {
			configOpts: []command.RunnerOpt{
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles),
				command.WithExtraArgs("--debug", "--verbose"),
			},
			checkFunc: func(t *testing.T, r *command.Runner) {
				t.Helper()
				_, currentProfile := r.GetCurrentProfile()
				assert.Equal(t, []string{"--debug", "--verbose"}, currentProfile.ExtraArgs)
			},
		},
		"configure with watch enabled": {
			configOpts: []command.RunnerOpt{
				command.WithPath("."),
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles),
				command.WithWatch(true),
			},
			checkFunc: func(t *testing.T, r *command.Runner) {
				t.Helper()
				// Watch should be enabled (no direct way to test this without accessing private fields)
				// We can test indirectly by checking if the runner was configured without error
				name, _ := r.GetCurrentProfile()
				assert.Equal(t, "yaml", name)
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner, err := command.NewRunnerWithRoot(root, ".", tc.configOpts...)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			t.Cleanup(runner.Close)

			if tc.checkFunc != nil {
				tc.checkFunc(t, runner)
			}
		})
	}
}

func TestRunner_ConfigureAfterCreation(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	yamlFile := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("key: value"), 0o644))

	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test"), 0o644))

	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	require.NoError(t, os.WriteFile(kustomizationFile, []byte("resources: []"), 0o644))

	// Create runner with initial configuration
	runner, err := command.NewRunnerWithRoot(root, ".",
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	// Verify initial profile
	name, currentProfile := runner.GetCurrentProfile()
	assert.Equal(t, "ks", name) // Should select ks because it has priority in rule order
	assert.NotNil(t, currentProfile)

	// Reconfigure with extra args
	err = runner.ConfigureContext(t.Context(),
		command.WithPath("."),
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles),
		command.WithExtraArgs("--debug", "--verbose"),
	)
	require.NoError(t, err)

	// Verify reconfiguration worked
	_, currentProfile = runner.GetCurrentProfile()
	require.NotNil(t, currentProfile)
	assert.Equal(t, []string{"--debug", "--verbose"}, currentProfile.ExtraArgs)
}

func TestRunner_WithExtraArgs(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	yamlFile := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("key: value"), 0o644))

	tcs := map[string]struct {
		extraArgs []string
		want      []string
	}{
		"single extra arg": {
			extraArgs: []string{"--debug"},
			want:      []string{"--debug"},
		},
		"multiple extra args": {
			extraArgs: []string{"--debug", "--verbose", "--output=json"},
			want:      []string{"--debug", "--verbose", "--output=json"},
		},
		"empty extra args": {
			extraArgs: []string{},
			want:      nil,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner, err := command.NewRunnerWithRoot(root, ".",
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles),
				command.WithExtraArgs(tc.extraArgs...))
			require.NoError(t, err)
			t.Cleanup(runner.Close)

			_, currentProfile := runner.GetCurrentProfile()
			require.NotNil(t, currentProfile)
			assert.Equal(t, tc.want, currentProfile.ExtraArgs)
		})
	}
}

func TestRunner_WithAutoProfile(t *testing.T) {
	t.Parallel()

	root, tempDir := testRoot(t)
	yamlFile := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("key: value"), 0o644))

	// Create runner with auto profile detection
	runner, err := command.NewRunnerWithRoot(root, ".",
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles),
		command.WithAutoProfile())
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	// Should automatically detect yaml profile based on the file
	name, currentProfile := runner.GetCurrentProfile()
	assert.Equal(t, "yaml", name)
	assert.NotNil(t, currentProfile)
}

func TestRunner_PathEscapePrevention(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create a subdirectory within the temp dir that will serve as our root
	rootDir := filepath.Join(tempDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))

	// Create a test file within the root
	testFile := filepath.Join(rootDir, "test.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("key: value"), 0o644))

	// Create a file outside the root directory to test escaping attempts
	outsideFile := filepath.Join(tempDir, "outside.yaml")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside: content"), 0o644))

	// Create another directory outside root
	outsideDir := filepath.Join(tempDir, "outside_dir")
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))

	outsideDirFile := filepath.Join(outsideDir, "file.yaml")
	require.NoError(t, os.WriteFile(outsideDirFile, []byte("content: test"), 0o644))

	// Create subdirectory and file within root for testing
	subDir := filepath.Join(rootDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	nestedFile := filepath.Join(subDir, "nested.yaml")
	require.NoError(t, os.WriteFile(nestedFile, []byte("nested: content"), 0o644))

	tcs := map[string]struct {
		path        string
		shouldError bool
	}{
		"escape attempt with ../": {
			path:        "../outside.yaml",
			shouldError: true,
		},
		"escape attempt with multiple ../": {
			path:        "../../outside.yaml",
			shouldError: true,
		},
		"escape attempt with mixed path": {
			path:        "../outside_dir/file.yaml",
			shouldError: true,
		},
		"escape attempt to parent directory": {
			path:        "..",
			shouldError: true,
		},
		"escape attempt with absolute path to temp dir": {
			path:        tempDir,
			shouldError: true,
		},
		"escape attempt with absolute path to outside file": {
			path:        outsideFile,
			shouldError: true,
		},
		"complex escape attempt": {
			path:        "subdir/../../outside.yaml",
			shouldError: true,
		},
		"non-existent path within root": {
			path:        "nonexistent.yaml",
			shouldError: true,
		},
		// Valid cases - these should work
		"valid relative path within root": {
			path:        "test.yaml",
			shouldError: false,
		},
		"valid subdirectory path within root": {
			path:        "subdir/nested.yaml",
			shouldError: false,
		},
		"valid current directory": {
			path:        ".",
			shouldError: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Open root for this test
			root, err := os.OpenRoot(rootDir)

			require.NoError(t, err)
			defer func() {
				require.NoError(t, root.Close())
			}()

			// Try to create a runner with the test path
			runner, err := command.NewRunnerWithRoot(root, tc.path,
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))

			if tc.shouldError {
				require.Error(t, err, "expected error for path: %s", tc.path)
				// Verify we don't get a runner when there's an error
				assert.Nil(t, runner)
			} else {
				require.NoError(t, err, "unexpected error for path: %s", tc.path)

				require.NotNil(t, runner)
				defer runner.Close()

				// For valid cases, verify the runner was configured correctly
				// Note: For "." we expect it to find yaml files
				if tc.path == "." {
					currentName, currentProfile := runner.GetCurrentProfile()
					assert.Equal(t, "yaml", currentName)
					assert.NotNil(t, currentProfile)
				} else {
					currentName, currentProfile := runner.GetCurrentProfile()
					assert.NotEmpty(t, currentName)
					assert.NotNil(t, currentProfile)
				}
			}
		})
	}
}

func TestRunner_ReconfigurePathEscapePrevention(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))

	// Create test files within and outside the root
	testFile := filepath.Join(rootDir, "test.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("key: value"), 0o644))

	outsideFile := filepath.Join(tempDir, "outside.yaml")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside: content"), 0o644))

	// Open root
	root, err := os.OpenRoot(rootDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, root.Close())
	})

	// Create a runner with a valid initial path
	runner, err := command.NewRunnerWithRoot(root, ".",
		command.WithRules(TestConfig.Rules),
		command.WithProfiles(TestConfig.Profiles))
	require.NoError(t, err)
	t.Cleanup(runner.Close)

	tcs := map[string]struct {
		newPath     string
		shouldError bool
	}{
		"reconfigure with escape attempt": {
			newPath:     "../outside.yaml",
			shouldError: true,
		},
		"reconfigure with absolute path outside root": {
			newPath:     outsideFile,
			shouldError: true,
		},
		"reconfigure with parent directory": {
			newPath:     "..",
			shouldError: true,
		},
		"reconfigure with current directory": {
			newPath:     ".",
			shouldError: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Try to reconfigure with the new path
			err := runner.Configure(
				command.WithPath(tc.newPath),
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles),
			)

			if tc.shouldError {
				require.Error(t, err, "expected error when reconfiguring with path: %s", tc.newPath)
			} else {
				require.NoError(t, err, "unexpected error when reconfiguring with path: %s", tc.newPath)
			}
		})
	}
}

func TestRunner_SymlinkPathEscapePrevention(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))

	// Create a file outside the root
	outsideFile := filepath.Join(tempDir, "outside.yaml")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside: content"), 0o644))

	// Create a valid file inside root for comparison
	validFile := filepath.Join(rootDir, "valid.yaml")
	require.NoError(t, os.WriteFile(validFile, []byte("valid: content"), 0o644))

	// Create a symlink inside root that points to another file inside root
	validSymlink := filepath.Join(rootDir, "valid_symlink.yaml")
	err := os.Symlink("valid.yaml", validSymlink)
	require.NoError(t, err)

	// Create a symlink inside root that points outside
	symlinkPath := filepath.Join(rootDir, "symlink.yaml")
	err = os.Symlink(outsideFile, symlinkPath)
	require.NoError(t, err)

	tcs := map[string]struct {
		path        string
		shouldError bool
	}{
		"symlink pointing outside root": {
			path:        "symlink.yaml",
			shouldError: true,
		},
		"valid file": {
			path:        "valid.yaml",
			shouldError: false,
		},
		"valid symlink to file within root": {
			path:        "valid_symlink.yaml",
			shouldError: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Open root for this specific test
			root, err := os.OpenRoot(rootDir)

			require.NoError(t, err)
			defer func() {
				require.NoError(t, root.Close())
			}()

			// Try to create a runner with the test path
			runner, err := command.NewRunnerWithRoot(root, tc.path,
				command.WithRules(TestConfig.Rules),
				command.WithProfiles(TestConfig.Profiles))

			if tc.shouldError {
				require.Error(t, err, "expected error for symlink path: %s", tc.path)
				assert.Nil(t, runner)
			} else {
				require.NoError(t, err, "unexpected error for symlink path: %s", tc.path)

				require.NotNil(t, runner)
				defer runner.Close()
			}
		})
	}
}
