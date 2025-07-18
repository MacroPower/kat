package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/profile"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		opts    []profile.ProfileOpt
		wantErr bool
	}{
		{
			name:    "valid profile",
			command: "echo",
			opts:    []profile.ProfileOpt{profile.WithArgs("hello")},
			wantErr: false,
		},
		{
			name:    "profile with source expression",
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithArgs("rendering"),
				profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			},
			wantErr: false,
		},
		{
			name:    "profile with extra args",
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithArgs("base", "command"),
				profile.WithExtraArgs("--verbose", "--output=json"),
			},
			wantErr: false,
		},
		{
			name:    "profile with args and extra args",
			command: "sh",
			opts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo template"),
				profile.WithExtraArgs("--debug", "--dry-run"),
			},
			wantErr: false,
		},
		{
			name:    "profile with invalid source expression",
			command: "echo",
			opts: []profile.ProfileOpt{
				profile.WithSource("invalid.expression()"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := profile.New(tt.command, tt.opts...)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, p)
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, tt.command, p.Command.Command)
			}
		})
	}
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	t.Run("valid profile", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("echo", profile.WithArgs("test"))
		require.NotNil(t, p)
		assert.Equal(t, "echo", p.Command.Command)
		assert.Equal(t, []string{"test"}, p.Command.Args)
	})

	t.Run("invalid profile panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			profile.MustNew("test", profile.WithSource("invalid.expression()"))
		})
	})
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

func TestProfile_Exec(t *testing.T) {
	t.Parallel()

	t.Run("successful execution", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("echo", profile.WithArgs("hello", "world"))
		result, err := p.Exec(t.Context(), "/tmp")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.Stdout, "hello world")
		assert.Empty(t, result.Stderr)
	})

	t.Run("failed execution", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("false") // command that always fails
		result, err := p.Exec(t.Context(), "/tmp")

		require.Error(t, err)
		assert.Nil(t, result)
		require.ErrorIs(t, err, execs.ErrCommandExecution)
	})

	t.Run("empty command", func(t *testing.T) {
		t.Parallel()

		p := &profile.Profile{} // empty command
		result, err := p.Exec(t.Context(), "/tmp")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "empty command")
	})
}

func TestProfile_WithHooks(t *testing.T) {
	t.Parallel()

	t.Run("successful pre-render hook", func(t *testing.T) {
		t.Parallel()

		hooks, err := profile.NewHooks(
			profile.WithPreRender(
				profile.MustNewHookCommand("echo", profile.WithHookArgs("pre-render")),
			),
		)
		require.NoError(t, err)

		p := profile.MustNew("echo",
			profile.WithArgs("main command"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "main command")
	})

	t.Run("failing pre-render hook", func(t *testing.T) {
		t.Parallel()

		hooks, err := profile.NewHooks(
			profile.WithPreRender(
				profile.MustNewHookCommand("false"), // always fails
			),
		)
		require.NoError(t, err)

		p := profile.MustNew("echo",
			profile.WithArgs("should not execute"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.Error(t, err)
		require.ErrorIs(t, err, profile.ErrHookExecution)
		assert.Nil(t, result) // main command should not have executed
	})

	t.Run("successful post-render hook", func(t *testing.T) {
		t.Parallel()

		hooks, err := profile.NewHooks(
			profile.WithPostRender(
				profile.MustNewHookCommand("grep", profile.WithHookArgs("main")),
			),
		)
		require.NoError(t, err)

		p := profile.MustNew("echo",
			profile.WithArgs("main command output"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "main command output")
	})
}

// TestNewHookCommandError tests that NewHookCommand returns an error for invalid configurations.
func TestNewHookCommandError(t *testing.T) {
	t.Parallel()

	// Test that invalid regex patterns in envFrom cause an error
	_, err := profile.NewHookCommand("echo",
		profile.WithHookArgs("test"),
		profile.WithHookEnvFrom([]execs.EnvFromSource{
			{
				CallerRef: &execs.CallerRef{
					Pattern: "[invalid regex",
				},
			},
		}),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook \"echo\"")
	assert.Contains(t, err.Error(), "envFrom[0]")
}

// TestMustNewHookCommandPanic tests that MustNewHookCommand panics on error.
func TestMustNewHookCommandPanic(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		profile.MustNewHookCommand("echo",
			profile.WithHookArgs("test"),
			profile.WithHookEnvFrom([]execs.EnvFromSource{
				{
					CallerRef: &execs.CallerRef{
						Pattern: "[invalid regex",
					},
				},
			}),
		)
	})
}

//nolint:paralleltest // Cannot use t.Parallel() because we use t.Setenv.
func TestProfile_EnvironmentIntegration(t *testing.T) {
	tcs := map[string]struct {
		setupEnv      func(t *testing.T)
		validateEnv   func(t *testing.T, result *execs.Result)
		profileOpts   []profile.ProfileOpt
		expectedError bool
	}{
		"static environment variable": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				// No OS environment setup needed.
			}, profileOpts: []profile.ProfileOpt{
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
			}, profileOpts: []profile.ProfileOpt{
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
		"envFrom with name reference": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("CUSTOM_VAR", "custom_value")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${CUSTOM_VAR:-not_found}"),
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "CUSTOM_VAR",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "custom_value")
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
		"caller reference to envFrom variable": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("SOURCE_VAR", "source_value")
			},
			profileOpts: []profile.ProfileOpt{
				profile.WithArgs("-c", "echo ${COPIED_VAR:-not_found}"),
				profile.WithEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "SOURCE_VAR",
						},
					},
				}),
				profile.WithEnvVar(execs.EnvVar{
					Name: "COPIED_VAR",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "SOURCE_VAR",
						},
					},
				}),
			},
			validateEnv: func(t *testing.T, result *execs.Result) {
				t.Helper()
				assert.Contains(t, result.Stdout, "source_value")
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
	}

	for name, tc := range tcs {
		//nolint:paralleltest // Cannot use t.Parallel() because we use t.Setenv.
		t.Run(name, func(t *testing.T) {
			tc.setupEnv(t)

			p, err := profile.New("sh", tc.profileOpts...)
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

//nolint:tparallel // Cannot use t.Parallel() because we use t.Setenv.
func TestProfile_EnvironmentWithHooks(t *testing.T) {
	//nolint:tparallel // Cannot use t.Parallel() because we use t.Setenv.
	t.Run("environment variables available in hooks", func(t *testing.T) {
		t.Setenv("HOOK_TEST_VAR", "hook_value")

		// Create a hook that uses an environment variable.
		preRenderHook, err := profile.NewHookCommand("sh",
			profile.WithHookArgs("-c", "echo ${HOOK_TEST_VAR:-not_found}"),
			profile.WithHookEnvFrom([]execs.EnvFromSource{
				{
					CallerRef: &execs.CallerRef{
						Name: "HOOK_TEST_VAR",
					},
				},
			}),
		)
		require.NoError(t, err)

		hooks, err := profile.NewHooks(
			profile.WithPreRender(preRenderHook),
		)
		require.NoError(t, err)

		p := profile.MustNew("echo",
			profile.WithArgs("main command"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "main command")
	})

	t.Run("hook with static environment variable", func(t *testing.T) {
		t.Parallel()

		// Create a hook with a static environment variable.
		postRenderHook, err := profile.NewHookCommand("sh",
			profile.WithHookArgs("-c", "echo ${HOOK_STATIC_VAR:-not_found}"),
			profile.WithHookEnvVar(execs.EnvVar{
				Name:  "HOOK_STATIC_VAR",
				Value: "hook_static_value",
			}),
		)
		require.NoError(t, err)

		hooks, err := profile.NewHooks(
			profile.WithPostRender(postRenderHook),
		)
		require.NoError(t, err)

		p := profile.MustNew("echo",
			profile.WithArgs("main output"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "main output")
	})
}

func TestProfile_ExtraArgsWithHooks(t *testing.T) {
	t.Parallel()

	t.Run("extra args applied before hooks execution", func(t *testing.T) {
		t.Parallel()

		// Create a post-render hook that processes the main command output
		postRenderHook, err := profile.NewHookCommand("grep", profile.WithHookArgs("world"))
		require.NoError(t, err)

		hooks, err := profile.NewHooks(
			profile.WithPostRender(postRenderHook),
		)
		require.NoError(t, err)

		// Create a profile with base args and extra args
		p := profile.MustNew("echo",
			profile.WithArgs("hello"),
			profile.WithExtraArgs("world", "from", "optional"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		// The main command should output "hello world from optional"
		// The post-render hook should grep for "world" and find it
		assert.Contains(t, result.Stdout, "hello world from optional")
	})

	t.Run("extra args with pre-render hooks", func(t *testing.T) {
		t.Parallel()

		// Create a pre-render hook
		preRenderHook, err := profile.NewHookCommand("echo", profile.WithHookArgs("pre-render executed"))
		require.NoError(t, err)

		hooks, err := profile.NewHooks(
			profile.WithPreRender(preRenderHook),
		)
		require.NoError(t, err)

		// Create a profile with extra args
		p := profile.MustNew("echo",
			profile.WithArgs("main"),
			profile.WithExtraArgs("command", "with", "args"),
			profile.WithHooks(hooks),
		)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		// Should contain output from main command with extra args
		assert.Contains(t, result.Stdout, "main command with args")
	})
}

func TestProfile_ExtraArgs(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expectedOut string
		input       []string
		want        []string
		baseArgs    []string
	}{
		"no extra args": {
			input:       nil,
			want:        nil,
			baseArgs:    []string{"hello"},
			expectedOut: "hello",
		},
		"single optional arg": {
			input:       []string{"world"},
			want:        []string{"world"},
			baseArgs:    []string{"hello"},
			expectedOut: "hello world",
		},
		"multiple extra args": {
			input:       []string{"beautiful", "world"},
			want:        []string{"beautiful", "world"},
			baseArgs:    []string{"hello"},
			expectedOut: "hello beautiful world",
		},
		"empty extra args list": {
			input:       []string{},
			want:        []string{},
			baseArgs:    []string{"hello"},
			expectedOut: "hello",
		},
		"extra args with flags": {
			input:       []string{"--flag", "value"},
			want:        []string{"--flag", "value"},
			baseArgs:    []string{"hello"},
			expectedOut: "hello --flag value",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := []profile.ProfileOpt{
				profile.WithArgs(tc.baseArgs...),
			}
			if tc.input != nil {
				opts = append(opts, profile.WithExtraArgs(tc.input...))
			}

			p, err := profile.New("echo", opts...)
			require.NoError(t, err)

			// Verify the profile was created with the correct extra args
			assert.Equal(t, tc.want, p.ExtraArgs)

			// Verify the args are used during execution
			result, err := p.Exec(t.Context(), "/tmp")
			require.NoError(t, err)
			assert.Contains(t, result.Stdout, tc.expectedOut)
		})
	}
}

func TestProfile_WithExtraArgs(t *testing.T) {
	t.Parallel()

	t.Run("profile creation with extra args", func(t *testing.T) {
		t.Parallel()

		p, err := profile.New("echo",
			profile.WithArgs("base"),
			profile.WithExtraArgs("--verbose", "--output=json"),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"--verbose", "--output=json"}, p.ExtraArgs)
	})

	t.Run("profile creation without extra args", func(t *testing.T) {
		t.Parallel()

		p, err := profile.New("echo", profile.WithArgs("base"))
		require.NoError(t, err)
		assert.Nil(t, p.ExtraArgs)
	})

	t.Run("extra args combined with other options", func(t *testing.T) {
		t.Parallel()

		p, err := profile.New("echo",
			profile.WithArgs("rendering", "files"),
			profile.WithExtraArgs("--verbose", "--format=yaml"),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"--verbose", "--format=yaml"}, p.ExtraArgs)
		assert.Equal(t, "echo", p.Command.Command)
		assert.Equal(t, []string{"rendering", "files"}, p.Command.Args)
		assert.NotEmpty(t, p.Source)
	})
}

func TestProfile_EnvironmentEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty environment variable name", func(t *testing.T) {
		t.Parallel()

		p, err := profile.New("sh",
			profile.WithArgs("-c", "echo ${EMPTY_NAME_VAR:-not_found}"),
			profile.WithEnvVar(execs.EnvVar{
				Name:  "", // Empty name should be ignored.
				Value: "should_not_appear",
			}),
		)
		require.NoError(t, err)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		// Since the empty name variable is ignored, the variable won't be set.
		assert.Contains(t, result.Stdout, "not_found")
	})

	t.Run("empty environment variable value", func(t *testing.T) {
		t.Parallel()

		p, err := profile.New("sh",
			profile.WithArgs("-c", "echo ${EMPTY_VAR:-not_found}"),
			profile.WithEnvVar(execs.EnvVar{
				Name:  "EMPTY_VAR",
				Value: "", // Empty value should be skipped.
			}),
		)
		require.NoError(t, err)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		// Should show the default value since the variable wasn't set due to empty value.
		assert.Contains(t, result.Stdout, "not_found")
	})

	t.Run("malformed base environment", func(t *testing.T) {
		t.Parallel()

		// This test verifies that the profile handles malformed base environment gracefully.
		// The NewEnvironment function in profile.go should handle this via os.Environ().
		p, err := profile.New("echo",
			profile.WithArgs("test"),
		)
		require.NoError(t, err)

		result, err := p.Exec(t.Context(), "/tmp")
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "test")
	})
}
