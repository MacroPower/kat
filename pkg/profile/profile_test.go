package profile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/profile"
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

// mockStatusManager is a test implementation of the StatusManager interface.
type mockStatusManager struct {
	renderResult profile.RenderResult
	renderStage  profile.RenderStage
}

func (m *mockStatusManager) SetError(_ context.Context) {
	// Not implemented for testing
}

func (m *mockStatusManager) SetResult(result profile.RenderResult) {
	m.renderResult = result
}

func (m *mockStatusManager) SetStage(stage profile.RenderStage) {
	m.renderStage = stage
}

func (m *mockStatusManager) RenderMap() map[string]any {
	return map[string]any{
		"stage":  int(m.renderStage),
		"result": string(m.renderResult),
	}
}

// newMockStatusManager creates a mock status manager with the specified stage and result.
func newMockStatusManager(stage profile.RenderStage, result profile.RenderResult) *mockStatusManager {
	return &mockStatusManager{
		renderStage:  stage,
		renderResult: result,
	}
}

func TestProfile_New(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		command string
		opts    []profile.ProfileOpt
		wantErr bool
	}{
		"valid profile": {
			command: "echo",
			opts:    []profile.ProfileOpt{profile.WithArgs("hello")},
			wantErr: false,
		},
		"profile with source expression": {
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithArgs("rendering"),
				profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			},
			wantErr: false,
		},
		"profile with extra args": {
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithArgs("base", "command"),
				profile.WithExtraArgs("--verbose", "--output=json"),
			},
			wantErr: false,
		},
		"profile with args and extra args": {
			command: "sh",
			opts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo template"),
				profile.WithExtraArgs("--debug", "--dry-run"),
			},
			wantErr: false,
		},
		"profile with invalid source expression": {
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithSource("invalid.expression()"),
			},
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name+" (New)", func(t *testing.T) {
			t.Parallel()

			p, err := profile.New(tc.command, tc.opts...)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, p)
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, tc.command, p.Command.Command)
			}
		})

		t.Run(name+" (MustNew)", func(t *testing.T) {
			t.Parallel()

			if tc.wantErr {
				assert.Panics(t, func() {
					profile.MustNew(tc.command, tc.opts...)
				})
			} else {
				p := profile.MustNew(tc.command, tc.opts...)
				require.NotNil(t, p)
				assert.Equal(t, tc.command, p.Command.Command)
			}
		})
	}
}

func TestProfile_Exec(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		executor    *mockExecutor
		wantResult  *execs.Result
		wantErr     error
		profileOpts []profile.ProfileOpt
	}{
		"successful execution": {
			executor: newMockExecutor("hello world\n", "", nil),
			wantResult: &execs.Result{
				Stdout: "hello world\n",
				Stderr: "",
			},
			wantErr: nil,
		},
		"execution with stderr": {
			executor: newMockExecutor("output", "warning message", nil),
			wantResult: &execs.Result{
				Stdout: "output",
				Stderr: "warning message",
			},
			wantErr: nil,
		},
		"failed execution": {
			executor:   newMockExecutor("", "", execs.ErrCommandExecution),
			wantResult: nil,
			wantErr:    execs.ErrCommandExecution,
		},
		"custom error": {
			executor:   newMockExecutor("", "", errors.New("custom error")),
			wantResult: nil,
			wantErr:    errors.New("custom error"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := append([]profile.ProfileOpt{}, tc.profileOpts...)
			opts = append(opts, profile.WithExecutor(tc.executor))

			p, err := profile.New("test", opts...)
			require.NoError(t, err)

			result, err := p.Exec(t.Context(), "/tmp")

			if tc.wantErr != nil {
				require.Error(t, err)
				if tc.wantErr.Error() != "" {
					assert.Contains(t, err.Error(), tc.wantErr.Error())
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.wantResult.Stdout, result.Stdout)
			assert.Equal(t, tc.wantResult.Stderr, result.Stderr)
		})
	}
}

//nolint:paralleltest // Cannot use t.Parallel() because we use t.Setenv.
func TestProfile_Environment(t *testing.T) {
	tcs := map[string]struct {
		setupEnv      func(t *testing.T)
		validateEnv   func(t *testing.T, result *execs.Result)
		profileOpts   []profile.ProfileOpt
		hookOpts      []profile.HookOpts
		expectedError bool
	}{
		"static environment variable": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// No OS environment setup needed.
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${STATIC_VAR:-not_found}"),
				profile.WithEnvVar(execs.EnvVar{
					Name:  "STATIC_VAR",
					Value: "static_value",
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "static_value")
			},
		},
		"envFrom with pattern matching": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("TEST_PATTERN_VAR1", "pattern_value1")
				t.Setenv("TEST_PATTERN_VAR2", "pattern_value2")
				t.Setenv("OTHER_VAR", "other_value")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${TEST_PATTERN_VAR1:-not_found}"),
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Pattern: "TEST_PATTERN_.*",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "pattern_value1")
			},
		},
		"environment variables in hooks": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("HOOK_TEST_VAR", "hook_value")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo main command"),
			},
			hookOpts: []profile.HookOpts{
				profile.WithPreRender(
					profile.MustNewHookCommand("echo",
						profile.WithHookArgs("pre-render done"),
						profile.WithHookEnvFrom([]execs.EnvFromSource{
							{
								CallerRef: &execs.CallerRef{
									Name: "HOOK_TEST_VAR",
								},
							},
						}),
					),
				),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "main command")
			},
		},
		"hook with static environment variable": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// No setup needed.
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo main output"),
			},
			hookOpts: []profile.HookOpts{
				profile.WithPostRender(
					profile.MustNewHookCommand("echo",
						profile.WithHookArgs("post-render done"),
						profile.WithHookEnvVar(execs.EnvVar{
							Name:  "HOOK_STATIC_VAR",
							Value: "hook_static_value",
						}),
					),
				),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "main output")
			},
		},
		"caller reference to essential variable": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("HOME", "/custom/home")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${COPIED_HOME:-not_found}"),
				profile.WithEnvVar(execs.EnvVar{
					Name: "COPIED_HOME",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "HOME",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "/custom/home")
			},
		},
		"static variable overrides envFrom": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("OVERRIDE_VAR", "from_env")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${OVERRIDE_VAR:-not_found}"),
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "OVERRIDE_VAR",
						},
					},
				}),
				profile.WithEnvVar(execs.EnvVar{
					Name:  "OVERRIDE_VAR",
					Value: "static_override",
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "static_override")
				assert.NotContains(t, result.Stdout, "from_env")
			},
		},
		"empty environment variable name": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// No setup needed.
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${EMPTY_NAME_VAR:-not_found}"),
				profile.WithEnvVar(execs.EnvVar{
					Name:  "", // Empty name should be ignored.
					Value: "should_not_appear",
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				// Since the empty name variable is ignored, the variable won't be set.
				assert.Contains(t, result.Stdout, "not_found")
			},
		},
		"empty environment variable value": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// No setup needed.
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${EMPTY_VAR:-not_found}"),
				profile.WithEnvVar(execs.EnvVar{
					Name:  "EMPTY_VAR",
					Value: "", // Empty value should be skipped.
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				// Should show the default value since the variable wasn't set due to empty value.
				assert.Contains(t, result.Stdout, "not_found")
			},
		},
		"missing caller reference": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// Don't set NONEXISTENT_VAR.
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${MISSING_VAR:-not_found}"),
				profile.WithEnvVar(execs.EnvVar{
					Name: "MISSING_VAR",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "NONEXISTENT_VAR",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				// The variable should not be set, so should show the default value.
				assert.Contains(t, result.Stdout, "not_found")
			},
		},
		"complex scenario with multiple env sources": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("HOME", "/test/home")
				t.Setenv("USER", "testuser")
				t.Setenv("PATTERN_VAR1", "pattern1")
				t.Setenv("PATTERN_VAR2", "pattern2")
				t.Setenv("NAMED_VAR", "named_value")
				t.Setenv("NON_ESSENTIAL", "should_not_appear")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs(
					"-c",
					"echo ${STATIC_VAR:-not_found} ${COPIED_HOME:-not_found} ${PATTERN_VAR1:-not_found} ${NAMED_VAR:-not_found}",
				),
				// Static variable.
				profile.WithEnvVar(execs.EnvVar{
					Name:  "STATIC_VAR",
					Value: "static",
				}),
				// Caller reference to essential variable.
				profile.WithEnvVar(execs.EnvVar{
					Name: "COPIED_HOME",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "HOME",
						},
					},
				}),
				// EnvFrom with pattern.
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Pattern: "PATTERN_.*",
						},
					},
				}),
				// EnvFrom with name.
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "NAMED_VAR",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "static")
				assert.Contains(t, result.Stdout, "/test/home")
				assert.Contains(t, result.Stdout, "pattern1")
				assert.Contains(t, result.Stdout, "named_value")
				// Non-essential vars should not appear.
				assert.NotContains(t, result.Stdout, "should_not_appear")
			},
		},
	}

	for name, tc := range tcs {
		//nolint:paralleltest // Cannot use t.Parallel() because we use t.Setenv.
		t.Run(name, func(t *testing.T) {
			tc.setupEnv(t)

			opts := append([]profile.ProfileOpt{}, tc.profileOpts...)

			// Add hooks if specified
			if len(tc.hookOpts) > 0 {
				hooks, err := profile.NewHooks(tc.hookOpts...)
				require.NoError(t, err)

				opts = append(opts, profile.WithHooks(hooks))
			}

			p, err := profile.New("sh", opts...)
			require.NoError(t, err)

			result, err := p.Exec(t.Context(), "/tmp")

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				tc.validateEnv(t, result)
			}
		})
	}
}

func TestProfile_MatchFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		source        string
		files         []string
		expectedFiles []string
		expectedMatch bool
	}{
		{
			name:          "no source expression",
			source:        "",
			files:         []string{"/app/test.yaml", "/app/config.json"},
			expectedMatch: true,
			expectedFiles: nil, // nil means use default filtering
		},
		{
			name:          "filter yaml files",
			source:        `files.filter(f, pathExt(f) in [".yaml", ".yml"])`,
			files:         []string{"/app/test.yaml", "/app/config.json", "/app/service.yml"},
			expectedMatch: true,
			expectedFiles: []string{"/app/test.yaml", "/app/service.yml"},
		},
		{
			name:          "no matches",
			source:        `files.filter(f, pathExt(f) == ".xml")`,
			files:         []string{"/app/test.yaml", "/app/config.json"},
			expectedMatch: false,
			expectedFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []profile.ProfileOpt{}
			if tt.source != "" {
				opts = append(opts, profile.WithSource(tt.source))
			}

			p, err := profile.New("test", opts...)
			require.NoError(t, err)

			match, files := p.MatchFiles("/app", tt.files)
			assert.Equal(t, tt.expectedMatch, match)
			if tt.expectedFiles != nil {
				assert.ElementsMatch(t, tt.expectedFiles, files)
			} else {
				assert.Nil(t, files)
			}
		})
	}
}

func TestProfile_MatchFileEvent(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err          error
		reload       string
		filePath     string
		renderResult profile.RenderResult
		renderStage  profile.RenderStage
		event        fsnotify.Op
		want         bool
	}{
		"no reload expression always returns true": {
			reload:       "",
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"simple file match": {
			reload:       `pathBase(file) == "config.yaml"`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"simple file no match": {
			reload:       `pathBase(file) == "config.yaml"`,
			filePath:     "/app/other.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"event filtering - match WRITE": {
			reload:       `fs.event.has(fs.WRITE)`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"event filtering - no match CREATE": {
			reload:       `fs.event.has(fs.WRITE)`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Create,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"event in list": {
			reload:       `fs.event.has(fs.WRITE, fs.RENAME)`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Rename,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"event not in list": {
			reload:       `fs.event.has(fs.WRITE, fs.RENAME)`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Remove,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"skip kustomization.yaml files": {
			reload:       `pathBase(file) != "kustomization.yaml"`,
			filePath:     "/app/kustomization.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"allow non-kustomization files": {
			reload:       `pathBase(file) != "kustomization.yaml"`,
			filePath:     "/app/deployment.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"status render.stage check - default empty stage": {
			reload:       `render.stage == render.STAGE_NONE`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"status render.stage check - under render stage": {
			reload:       `render.stage < render.STAGE_RENDER`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StagePreRender,
			renderResult: profile.ResultNone,
		},
		"status render.stage check - at render stage": {
			reload:       `render.stage < render.STAGE_RENDER`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageRender,
			renderResult: profile.ResultNone,
		},
		"status render.stage check - post render stage": {
			reload:       `render.stage < render.STAGE_RENDER`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StagePostRender,
			renderResult: profile.ResultNone,
		},
		"status render.result check - default empty result": {
			reload:       `render.result == render.RESULT_NONE`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"status render.result check - not CANCEL": {
			reload:       `render.result != render.RESULT_CANCEL`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultOK,
		},
		"status render.result check - is CANCEL": {
			reload:       `render.result != render.RESULT_CANCEL`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultCancel,
		},
		"status render.result check - is ERROR": {
			reload:       `render.result == render.RESULT_ERROR`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultError,
		},
		"complex expression with multiple conditions": {
			reload: `
				pathBase(file) != "kustomization.yaml" &&
				fs.event.has(fs.WRITE, fs.RENAME) &&
				render.result != render.RESULT_CANCEL`,
			filePath:     "/app/deployment.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultOK,
		},
		"complex expression fails on kustomization": {
			reload: `
				pathBase(file) != "kustomization.yaml" &&
				fs.event.has(fs.WRITE, fs.RENAME) &&
				render.result != render.RESULT_CANCEL`,
			filePath:     "/app/kustomization.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultOK,
		},
		"complex expression fails on wrong event": {
			reload: `
				pathBase(file) != "kustomization.yaml" &&
				fs.event.has(fs.WRITE, fs.RENAME) &&
				render.result != render.RESULT_CANCEL`,
			filePath:     "/app/deployment.yaml",
			event:        fsnotify.Remove,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultOK,
		},
		"complex expression fails on CANCEL result": {
			reload: `
				pathBase(file) != "kustomization.yaml" &&
				fs.event.has(fs.WRITE, fs.RENAME) &&
				render.result != render.RESULT_CANCEL`,
			filePath:     "/app/deployment.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultCancel,
		},
		"file extension filtering": {
			reload:       `pathExt(file) == ".yaml"`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         true,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"file extension no match": {
			reload:       `pathExt(file) == ".yaml"`,
			filePath:     "/app/config.json",
			event:        fsnotify.Write,
			want:         false,
			err:          nil,
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
		"expression returning non-boolean": {
			reload:       `"not a boolean"`,
			filePath:     "/app/config.yaml",
			event:        fsnotify.Write,
			want:         false,
			err:          errors.New("reload expression did not return a boolean value"),
			renderStage:  profile.StageNone,
			renderResult: profile.ResultNone,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := []profile.ProfileOpt{}
			if tc.reload != "" {
				opts = append(opts, profile.WithReload(tc.reload))
			}

			// Add mock status manager with configured render stage and result
			mockStatus := newMockStatusManager(tc.renderStage, tc.renderResult)
			opts = append(opts, profile.WithStatusManager(mockStatus))

			p, err := profile.New("echo", opts...)
			require.NoError(t, err)

			got, err := p.MatchFileEvent(tc.filePath, tc.event)

			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
