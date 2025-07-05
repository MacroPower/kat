package execs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/execs"
)

func TestNewCommand(t *testing.T) {
	t.Parallel()

	baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test"}
	env := execs.NewCommand(baseEnv)
	assert.NotNil(t, env)
	assert.Empty(t, env.Env)
	assert.Empty(t, env.EnvFrom)
}

func TestCommand_AddEnvVar(t *testing.T) {
	t.Parallel()

	env := execs.NewCommand([]string{})
	envVar := execs.EnvVar{
		Name:  "TEST_VAR",
		Value: "test_value",
	}

	env.AddEnvVar(envVar)

	assert.Len(t, env.Env, 1)
	assert.Equal(t, "TEST_VAR", env.Env[0].Name)
	assert.Equal(t, "test_value", env.Env[0].Value)
}

func TestCommand_AddEnvFrom(t *testing.T) {
	t.Parallel()

	env := execs.NewCommand([]string{})
	callerRef := &execs.CallerRef{
		Name: "HOME",
	}
	envFrom := []execs.EnvFromSource{
		{CallerRef: callerRef},
	}

	env.AddEnvFrom(envFrom)

	assert.Len(t, env.EnvFrom, 1)
	assert.Equal(t, "HOME", env.EnvFrom[0].CallerRef.Name)
}

func TestCommand_SetBaseEnv(t *testing.T) {
	t.Parallel()

	env := execs.NewCommand([]string{"INITIAL=value"})

	// Change the base environment.
	newBaseEnv := []string{"NEW_VAR=new_value", "PATH=/usr/bin"}
	env.SetBaseEnv(newBaseEnv)

	result := env.GetEnv()

	// Should contain the new base environment variables (essential ones).
	assert.Contains(t, result, "PATH=/usr/bin")
	// Should not contain the initial variable since it's not essential.
	for _, envVar := range result {
		assert.NotContains(t, envVar, "INITIAL=value")
	}
}

func TestCommand_Build(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupEnv func() execs.Command
		validate func(t *testing.T, result []string)
	}{
		"empty environment with base env": {
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test", "NON_ESSENTIAL=value"}

				return execs.NewCommand(baseEnv)
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should contain essential vars from base env.
				assert.Contains(t, result, "PATH=/usr/bin")
				assert.Contains(t, result, "HOME=/home/test")
				// Should not contain non-essential vars.
				for _, envVar := range result {
					assert.NotContains(t, envVar, "NON_ESSENTIAL=value")
				}
			},
		},
		"static environment variable": {
			setupEnv: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name:  "STATIC_VAR",
					Value: "static_value",
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				assert.Contains(t, result, "STATIC_VAR=static_value")
			},
		},
		"environment variable from caller reference - nonexistent": {
			setupEnv: func() execs.Command {
				// Test case where the caller reference doesn't exist in baseEnv or envMap.
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name: "FROM_CALLER",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "NONEXISTENT_CALLER_VAR",
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should not contain the variable since the caller reference doesn't exist.
				for _, env := range result {
					assert.NotContains(t, env, "FROM_CALLER=")
				}
			},
		},
		"environment variable from caller reference - exists in base": {
			setupEnv: func() execs.Command {
				// Test with a caller reference to an essential variable.
				baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test"}
				env := execs.NewCommand(baseEnv)
				env.AddEnvVar(execs.EnvVar{
					Name: "FROM_CALLER",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "PATH", // PATH is essential, so it will be in envMap.
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should contain the variable with the value from essential variable.
				assert.Contains(t, result, "FROM_CALLER=/usr/bin")
			},
		},
		"environment variable from caller reference - PATH": {
			setupEnv: func() execs.Command {
				t.Helper()
				// Test referencing PATH which is provided in base environment.
				baseEnv := []string{"PATH=/usr/bin:/bin"}
				env := execs.NewCommand(baseEnv)
				env.AddEnvVar(execs.EnvVar{
					Name: "COPIED_PATH",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "PATH",
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should contain COPIED_PATH with PATH's value from the base environment.
				assert.Contains(t, result, "COPIED_PATH=/usr/bin:/bin")
				// PATH should also be present since it's essential.
				assert.Contains(t, result, "PATH=/usr/bin:/bin")
			},
		},
		"envFrom with name reference": {
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{"TEST_ENVFROM_VAR=envfrom_value", "PATH=/usr/bin"}
				env := execs.NewCommand(baseEnv)
				env.AddEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "TEST_ENVFROM_VAR",
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				assert.Contains(t, result, "TEST_ENVFROM_VAR=envfrom_value")
			},
		},
		"envFrom with pattern reference": {
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{
					"TEST_PATTERN_VAR1=pattern_value1",
					"TEST_PATTERN_VAR2=pattern_value2",
					"OTHER_VAR=other_value",
					"PATH=/usr/bin",
				}
				env := execs.NewCommand(baseEnv)
				callerRef := &execs.CallerRef{
					Pattern: "TEST_PATTERN_.*",
				}
				// Compile the pattern.
				err := callerRef.Compile()
				require.NoError(t, err)

				env.AddEnvFrom([]execs.EnvFromSource{
					{CallerRef: callerRef},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				assert.Contains(t, result, "TEST_PATTERN_VAR1=pattern_value1")
				assert.Contains(t, result, "TEST_PATTERN_VAR2=pattern_value2")
				// OTHER_VAR should not be included since it doesn't match the pattern.
				for _, envVar := range result {
					assert.NotContains(t, envVar, "OTHER_VAR=other_value")
				}
			},
		},
		"static variable overrides base environment": {
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{"OVERRIDE_VAR=old_value", "PATH=/usr/bin"}
				env := execs.NewCommand(baseEnv)
				env.AddEnvVar(execs.EnvVar{
					Name:  "OVERRIDE_VAR",
					Value: "new_value",
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				assert.Contains(t, result, "OVERRIDE_VAR=new_value")
				// Should not contain the old value.
				for _, envVar := range result {
					assert.NotContains(t, envVar, "OVERRIDE_VAR=old_value")
				}
			},
		},
		"missing caller reference": {
			setupEnv: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name: "MISSING_REF",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Name: "NONEXISTENT_VAR",
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should not contain the variable since the reference doesn't exist.
				for _, env := range result {
					assert.NotContains(t, env, "MISSING_REF=")
				}
			},
		},
		"envFrom with missing name reference": {
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{"PATH=/usr/bin"}
				env := execs.NewCommand(baseEnv)
				env.AddEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Name: "NONEXISTENT_BASE_VAR",
						},
					},
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Should not contain the nonexistent variable.
				for _, envVar := range result {
					assert.NotContains(t, envVar, "NONEXISTENT_BASE_VAR=")
				}
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := tc.setupEnv()
			result := env.GetEnv()
			tc.validate(t, result)
		})
	}
}

func TestCommand_CompilePatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup   func() execs.Command
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "no patterns to compile",
			setup: func() execs.Command {
				t.Helper()

				return execs.NewCommand([]string{})
			},
			wantErr: false,
		},
		{
			name: "valid env pattern",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name: "TEST_VAR",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Pattern: "TEST_.*",
						},
					},
				})

				return env
			},
			wantErr: false,
		},
		{
			name: "valid envFrom pattern",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Pattern: "ENV_.*",
						},
					},
				})

				return env
			},
			wantErr: false,
		},
		{
			name: "invalid env pattern",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name: "TEST_VAR",
					ValueFrom: &execs.EnvVarSource{
						CallerRef: &execs.CallerRef{
							Pattern: "[invalid",
						},
					},
				})

				return env
			},
			wantErr: true,
			errMsg:  "env[0]",
		},
		{
			name: "invalid envFrom pattern",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvFrom([]execs.EnvFromSource{
					{
						CallerRef: &execs.CallerRef{
							Pattern: "[invalid",
						},
					},
				})

				return env
			},
			wantErr: true,
			errMsg:  "envFrom[0]",
		},
		{
			name: "nil caller ref in env",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name:      "TEST_VAR",
					ValueFrom: &execs.EnvVarSource{},
				})

				return env
			},
			wantErr: false,
		},
		{
			name: "nil caller ref in envFrom",
			setup: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvFrom([]execs.EnvFromSource{
					{CallerRef: nil},
				})

				return env
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := tt.setup()
			err := env.CompilePatterns()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommand_Build_EssentialVars(t *testing.T) {
	t.Parallel()

	// Create a base environment with both essential and non-essential vars.
	baseEnv := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/test",
		"USER=testuser",
		"TERM=xterm",
		"COLORTERM=truecolor",
		"NON_ESSENTIAL=should_not_appear",
	}

	env := execs.NewCommand(baseEnv)
	result := env.GetEnv()

	// Essential variables should be present.
	essentialVars := []string{"PATH", "HOME", "USER", "TERM", "COLORTERM"}
	for _, essential := range essentialVars {
		found := false
		for _, envVar := range result {
			if len(envVar) > len(essential)+1 && envVar[:len(essential)+1] == essential+"=" {
				found = true

				break
			}
		}
		assert.True(t, found, "Essential variable %s should be present", essential)
	}

	// Non-essential variables should not be present.
	for _, envVar := range result {
		assert.NotContains(t, envVar, "NON_ESSENTIAL=should_not_appear")
	}
}

func TestCommand_Build_EmptyVariableName(t *testing.T) {
	t.Parallel()

	env := execs.NewCommand([]string{})
	env.AddEnvVar(execs.EnvVar{
		Name:  "", // Empty name should be skipped.
		Value: "some_value",
	})

	result := env.GetEnv()

	// Should not contain any variable with empty name.
	for _, envVar := range result {
		assert.NotEqual(t, "=some_value", envVar)
	}
}

func TestCommand_Build_ComplexScenario(t *testing.T) {
	t.Parallel()

	// Create a complex base environment.
	baseEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/test",
		"CALLER_VAR1=caller1",
		"CALLER_VAR2=caller2",
		"PATTERN_TEST_VAR=pattern_test",
		"BASE_VAR=base_value",
	}

	env := execs.NewCommand(baseEnv)

	// Add static env var.
	env.AddEnvVar(execs.EnvVar{
		Name:  "STATIC",
		Value: "static_value",
	})

	// Add env var from caller reference (now uses envMap lookup).
	env.AddEnvVar(execs.EnvVar{
		Name: "FROM_CALLER",
		ValueFrom: &execs.EnvVarSource{
			CallerRef: &execs.CallerRef{
				Name: "HOME", // HOME is essential, so it will be in envMap.
			},
		},
	})

	// Add envFrom with name.
	env.AddEnvFrom([]execs.EnvFromSource{
		{
			CallerRef: &execs.CallerRef{
				Name: "CALLER_VAR2",
			},
		},
	})

	// Add envFrom with pattern.
	patternRef := &execs.CallerRef{
		Pattern: "PATTERN_.*",
	}
	err := patternRef.Compile()
	require.NoError(t, err)

	env.AddEnvFrom([]execs.EnvFromSource{
		{CallerRef: patternRef},
	})

	result := env.GetEnv()

	// Verify all expected variables are present.
	assert.Contains(t, result, "PATH=/usr/bin")
	assert.Contains(t, result, "HOME=/home/test")
	assert.Contains(t, result, "STATIC=static_value")
	assert.Contains(t, result, "FROM_CALLER=/home/test")
	assert.Contains(t, result, "CALLER_VAR2=caller2")
	assert.Contains(t, result, "PATTERN_TEST_VAR=pattern_test")

	// BASE_VAR should not be present since it's not essential.
	for _, envVar := range result {
		assert.NotContains(t, envVar, "BASE_VAR=base_value")
	}
}

func TestCommand_Build_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupEnv func() execs.Command
		validate func(t *testing.T, result []string)
		name     string
	}{
		{
			name: "base environment with malformed entry",
			setupEnv: func() execs.Command {
				t.Helper()
				baseEnv := []string{"VALID_VAR=value", "MALFORMED_NO_EQUALS", "PATH=/usr/bin", "ANOTHER_VALID=another"}

				return execs.NewCommand(baseEnv)
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				assert.Contains(t, result, "PATH=/usr/bin")
				// Malformed entry should be ignored in base env parsing.
				found := false
				for _, env := range result {
					if env == "MALFORMED_NO_EQUALS" {
						found = true

						break
					}
				}
				assert.False(t, found, "Malformed environment variable should not be included")
			},
		},
		{
			name: "environment variable with empty value",
			setupEnv: func() execs.Command {
				t.Helper()
				env := execs.NewCommand([]string{})
				env.AddEnvVar(execs.EnvVar{
					Name:  "EMPTY_VAR",
					Value: "",
				})

				return env
			},
			validate: func(t *testing.T, result []string) {
				t.Helper()
				// Empty values are skipped in applyEnv, so should not be present.
				for _, env := range result {
					assert.NotContains(t, env, "EMPTY_VAR=")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := tt.setupEnv()
			result := env.GetEnv()
			tt.validate(t, result)
		})
	}
}

func TestCommand_Build_ApplyEnvFromEdgeCases(t *testing.T) {
	t.Parallel()

	// Test case with pattern matching against base environment.
	baseEnv := []string{
		"TEST_MATCH_VAR1=value1",
		"TEST_MATCH_VAR2=value2",
		"NO_MATCH_VAR=no_value",
		"PATH=/usr/bin",
	}

	env := execs.NewCommand(baseEnv)

	// Set up a pattern that would match TEST_MATCH_.
	callerRef := &execs.CallerRef{
		Pattern: "TEST_MATCH_.*",
	}
	err := callerRef.Compile()
	require.NoError(t, err)

	env.AddEnvFrom([]execs.EnvFromSource{
		{CallerRef: callerRef},
	})

	result := env.GetEnv()

	// Should contain the matched variables.
	assert.Contains(t, result, "TEST_MATCH_VAR1=value1")
	assert.Contains(t, result, "TEST_MATCH_VAR2=value2")

	// Should not contain the non-matching variable (since it's not essential).
	for _, envVar := range result {
		assert.NotContains(t, envVar, "NO_MATCH_VAR=no_value")
	}
}

func TestCommand_Build_EnvFromMakesVariableAvailableForCallerRef(t *testing.T) {
	t.Parallel()

	// Test that envFrom can make non-essential variables available for caller references.
	baseEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/test",
		"CUSTOM_VAR=custom_value", // This is not essential, but will be added via envFrom.
	}

	env := execs.NewCommand(baseEnv)

	// First, add the non-essential variable via envFrom.
	env.AddEnvFrom([]execs.EnvFromSource{
		{
			CallerRef: &execs.CallerRef{
				Name: "CUSTOM_VAR",
			},
		},
	})

	// Then, add an env var that references it via caller reference.
	env.AddEnvVar(execs.EnvVar{
		Name: "COPIED_CUSTOM",
		ValueFrom: &execs.EnvVarSource{
			CallerRef: &execs.CallerRef{
				Name: "CUSTOM_VAR",
			},
		},
	})

	result := env.GetEnv()

	// Both the original and the copied variable should be present.
	assert.Contains(t, result, "CUSTOM_VAR=custom_value")
	assert.Contains(t, result, "COPIED_CUSTOM=custom_value")
}

func TestCommand_Exec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		errType  error
		setup    func() execs.Command
		validate func(t *testing.T, result *execs.Result, err error)
		name     string
		dir      string
		wantErr  bool
	}{
		{
			name: "successful command execution",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "echo"
				cmd.Args = []string{"hello", "world"}

				return cmd
			},
			dir:     "",
			wantErr: false,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "hello world\n", result.Stdout)
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "command with working directory",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "pwd"

				return cmd
			},
			dir:     "/tmp",
			wantErr: false,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Contains(t, result.Stdout, "/tmp")
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "command with environment variables",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "sh"
				cmd.Args = []string{"-c", "echo $TEST_VAR"}
				cmd.AddEnvVar(execs.EnvVar{
					Name:  "TEST_VAR",
					Value: "test_value",
				})

				return cmd
			},
			dir:     "",
			wantErr: false,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "test_value\n", result.Stdout)
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "empty command",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = ""

				return cmd
			},
			dir:     "",
			wantErr: true,
			errType: execs.ErrEmptyCommand,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorIs(t, err, execs.ErrCommandExecution)
				require.ErrorIs(t, err, execs.ErrEmptyCommand)
				assert.Nil(t, result)
			},
		},
		{
			name: "nonexistent command",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "nonexistent-command-12345"

				return cmd
			},
			dir:     "",
			wantErr: true,
			errType: execs.ErrCommandExecution,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorIs(t, err, execs.ErrCommandExecution)
				assert.Nil(t, result)
			},
		},
		{
			name: "command with stderr output",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "sh"
				cmd.Args = []string{"-c", "echo 'stdout output'; echo 'stderr output' >&2"}

				return cmd
			},
			dir:     "",
			wantErr: false,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "stdout output\n", result.Stdout)
				assert.Equal(t, "stderr output\n", result.Stderr)
			},
		},
		{
			name: "command with non-zero exit code but output",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "sh"
				cmd.Args = []string{"-c", "echo 'some output'; exit 1"}

				return cmd
			},
			dir:     "",
			wantErr: true,
			errType: execs.ErrCommandExecution,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorIs(t, err, execs.ErrCommandExecution)
				// Should return result with output even on error.
				require.NotNil(t, result)
				assert.Equal(t, "some output\n", result.Stdout)
			},
		},
		{
			name: "command with non-zero exit code and stderr",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "sh"
				cmd.Args = []string{"-c", "echo 'error message' >&2; exit 2"}

				return cmd
			},
			dir:     "",
			wantErr: true,
			errType: execs.ErrCommandExecution,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorIs(t, err, execs.ErrCommandExecution)
				// Should return result with output even on error.
				require.NotNil(t, result)
				assert.Empty(t, result.Stdout)
				assert.Equal(t, "error message\n", result.Stderr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.setup()
			ctx := t.Context()
			result, err := cmd.Exec(ctx, tt.dir)

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestCommand_ExecWithStdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func() execs.Command
		validate func(t *testing.T, result *execs.Result, err error)
		name     string
		dir      string
		stdin    []byte
		wantErr  bool
	}{
		{
			name: "command with stdin input",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "cat"

				return cmd
			},
			stdin: []byte("hello from stdin"),
			dir:   "",
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "hello from stdin", result.Stdout)
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "command processing stdin with grep",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "grep"
				cmd.Args = []string{"test"}

				return cmd
			},
			stdin: []byte("line 1\ntest line\nline 3\nanother test\n"),
			dir:   "",
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Contains(t, result.Stdout, "test line")
				assert.Contains(t, result.Stdout, "another test")
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "command with empty stdin",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "wc"
				cmd.Args = []string{"-l"}

				return cmd
			},
			stdin: []byte{},
			dir:   "",
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Contains(t, result.Stdout, "0")
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "command with nil stdin",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
				cmd.Command = "echo"
				cmd.Args = []string{"test"}

				return cmd
			},
			stdin: nil,
			dir:   "",
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "test\n", result.Stdout)
				assert.Empty(t, result.Stderr)
			},
		},
		{
			name: "empty command with stdin",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = ""

				return cmd
			},
			stdin:   []byte("some input"),
			dir:     "",
			wantErr: true,
			validate: func(t *testing.T, result *execs.Result, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorIs(t, err, execs.ErrCommandExecution)
				require.ErrorIs(t, err, execs.ErrEmptyCommand)
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.setup()
			ctx := t.Context()
			result, err := cmd.ExecWithStdin(ctx, tt.dir, tt.stdin)

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestCommand_ExecWithContext(t *testing.T) {
	t.Parallel()

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		cmd := execs.NewCommand([]string{"PATH=/usr/bin:/bin"})
		cmd.Command = "sleep"
		cmd.Args = []string{"10"}

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		result, err := cmd.Exec(ctx, "")

		require.Error(t, err)
		assert.ErrorIs(t, err, execs.ErrCommandExecution)
		// May or may not have result depending on timing.
		_ = result
	})
}

func TestCommand_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() execs.Command
		expected string
	}{
		{
			name: "command without arguments",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = "echo"

				return cmd
			},
			expected: "echo ",
		},
		{
			name: "command with single argument",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = "echo"
				cmd.Args = []string{"hello"}

				return cmd
			},
			expected: "echo hello",
		},
		{
			name: "command with multiple arguments",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = "git"
				cmd.Args = []string{"commit", "-m", "test message"}

				return cmd
			},
			expected: "git commit -m test message",
		},
		{
			name: "command with arguments containing spaces",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = "echo"
				cmd.Args = []string{"hello world", "test"}

				return cmd
			},
			expected: "echo hello world test",
		},
		{
			name: "empty command",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = ""

				return cmd
			},
			expected: " ",
		},
		{
			name: "command with empty arguments",
			setup: func() execs.Command {
				cmd := execs.NewCommand([]string{})
				cmd.Command = "test"
				cmd.Args = []string{"", "arg2", ""}

				return cmd
			},
			expected: "test  arg2 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.setup()
			result := cmd.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommand_ApplyEnvFrom_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("envFrom with nil CallerRef", func(t *testing.T) {
		t.Parallel()

		baseEnv := []string{"PATH=/usr/bin", "TEST_VAR=test_value"}
		cmd := execs.NewCommand(baseEnv)
		cmd.AddEnvFrom([]execs.EnvFromSource{
			{CallerRef: nil}, // This should be skipped.
		})

		result := cmd.GetEnv()

		// Should only contain essential variables.
		assert.Contains(t, result, "PATH=/usr/bin")
		// Should not contain TEST_VAR since CallerRef is nil.
		for _, env := range result {
			assert.NotContains(t, env, "TEST_VAR=test_value")
		}
	})

	t.Run("envFrom with empty pattern and name", func(t *testing.T) {
		t.Parallel()

		baseEnv := []string{"PATH=/usr/bin", "TEST_VAR=test_value"}
		cmd := execs.NewCommand(baseEnv)
		cmd.AddEnvFrom([]execs.EnvFromSource{
			{
				CallerRef: &execs.CallerRef{
					Pattern: "", // Empty pattern.
					Name:    "", // Empty name.
				},
			},
		})

		result := cmd.GetEnv()

		// Should only contain essential variables.
		assert.Contains(t, result, "PATH=/usr/bin")
		// Should not contain TEST_VAR.
		for _, env := range result {
			assert.NotContains(t, env, "TEST_VAR=test_value")
		}
	})

	t.Run("envFrom with pattern that matches no variables", func(t *testing.T) {
		t.Parallel()

		baseEnv := []string{"PATH=/usr/bin", "TEST_VAR=test_value"}
		cmd := execs.NewCommand(baseEnv)

		callerRef := &execs.CallerRef{
			Pattern: "NO_MATCH_.*",
		}
		err := callerRef.Compile()
		require.NoError(t, err)

		cmd.AddEnvFrom([]execs.EnvFromSource{
			{CallerRef: callerRef},
		})

		result := cmd.GetEnv()

		// Should only contain essential variables.
		assert.Contains(t, result, "PATH=/usr/bin")
		// Should not contain TEST_VAR since pattern doesn't match.
		for _, env := range result {
			assert.NotContains(t, env, "TEST_VAR=test_value")
		}
	})
}
