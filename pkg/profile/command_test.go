package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/profile"
)

func TestNewCommand(t *testing.T) {
	t.Parallel()

	baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test"}
	env := profile.NewCommand(baseEnv)
	assert.NotNil(t, env)
	assert.Empty(t, env.Env)
	assert.Empty(t, env.EnvFrom)
}

func TestCommand_AddEnvVar(t *testing.T) {
	t.Parallel()

	env := profile.NewCommand([]string{})
	envVar := profile.EnvVar{
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

	env := profile.NewCommand([]string{})
	callerRef := &profile.CallerRef{
		Name: "HOME",
	}
	envFrom := []profile.EnvFromSource{
		{CallerRef: callerRef},
	}

	env.AddEnvFrom(envFrom)

	assert.Len(t, env.EnvFrom, 1)
	assert.Equal(t, "HOME", env.EnvFrom[0].CallerRef.Name)
}

func TestCommand_SetBaseEnv(t *testing.T) {
	t.Parallel()

	env := profile.NewCommand([]string{"INITIAL=value"})

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
		setupEnv func() profile.Command
		validate func(t *testing.T, result []string)
	}{
		"empty environment with base env": {
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test", "NON_ESSENTIAL=value"}

				return profile.NewCommand(baseEnv)
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
			setupEnv: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
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
			setupEnv: func() profile.Command {
				// Test case where the caller reference doesn't exist in baseEnv or envMap.
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
					Name: "FROM_CALLER",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setupEnv: func() profile.Command {
				// Test with a caller reference to an essential variable.
				baseEnv := []string{"PATH=/usr/bin", "HOME=/home/test"}
				env := profile.NewCommand(baseEnv)
				env.AddEnvVar(profile.EnvVar{
					Name: "FROM_CALLER",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setupEnv: func() profile.Command {
				t.Helper()
				// Test referencing PATH which is provided in base environment.
				baseEnv := []string{"PATH=/usr/bin:/bin"}
				env := profile.NewCommand(baseEnv)
				env.AddEnvVar(profile.EnvVar{
					Name: "COPIED_PATH",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{"TEST_ENVFROM_VAR=envfrom_value", "PATH=/usr/bin"}
				env := profile.NewCommand(baseEnv)
				env.AddEnvFrom([]profile.EnvFromSource{
					{
						CallerRef: &profile.CallerRef{
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
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{
					"TEST_PATTERN_VAR1=pattern_value1",
					"TEST_PATTERN_VAR2=pattern_value2",
					"OTHER_VAR=other_value",
					"PATH=/usr/bin",
				}
				env := profile.NewCommand(baseEnv)
				callerRef := &profile.CallerRef{
					Pattern: "TEST_PATTERN_.*",
				}
				// Compile the pattern.
				err := callerRef.Compile()
				require.NoError(t, err)

				env.AddEnvFrom([]profile.EnvFromSource{
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
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{"OVERRIDE_VAR=old_value", "PATH=/usr/bin"}
				env := profile.NewCommand(baseEnv)
				env.AddEnvVar(profile.EnvVar{
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
			setupEnv: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
					Name: "MISSING_REF",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{"PATH=/usr/bin"}
				env := profile.NewCommand(baseEnv)
				env.AddEnvFrom([]profile.EnvFromSource{
					{
						CallerRef: &profile.CallerRef{
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
		setup   func() profile.Command
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "no patterns to compile",
			setup: func() profile.Command {
				t.Helper()

				return profile.NewCommand([]string{})
			},
			wantErr: false,
		},
		{
			name: "valid env pattern",
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
					Name: "TEST_VAR",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvFrom([]profile.EnvFromSource{
					{
						CallerRef: &profile.CallerRef{
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
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
					Name: "TEST_VAR",
					ValueFrom: &profile.EnvVarSource{
						CallerRef: &profile.CallerRef{
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
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvFrom([]profile.EnvFromSource{
					{
						CallerRef: &profile.CallerRef{
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
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
					Name:      "TEST_VAR",
					ValueFrom: &profile.EnvVarSource{},
				})

				return env
			},
			wantErr: false,
		},
		{
			name: "nil caller ref in envFrom",
			setup: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvFrom([]profile.EnvFromSource{
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

	env := profile.NewCommand(baseEnv)
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

	env := profile.NewCommand([]string{})
	env.AddEnvVar(profile.EnvVar{
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

	env := profile.NewCommand(baseEnv)

	// Add static env var.
	env.AddEnvVar(profile.EnvVar{
		Name:  "STATIC",
		Value: "static_value",
	})

	// Add env var from caller reference (now uses envMap lookup).
	env.AddEnvVar(profile.EnvVar{
		Name: "FROM_CALLER",
		ValueFrom: &profile.EnvVarSource{
			CallerRef: &profile.CallerRef{
				Name: "HOME", // HOME is essential, so it will be in envMap.
			},
		},
	})

	// Add envFrom with name.
	env.AddEnvFrom([]profile.EnvFromSource{
		{
			CallerRef: &profile.CallerRef{
				Name: "CALLER_VAR2",
			},
		},
	})

	// Add envFrom with pattern.
	patternRef := &profile.CallerRef{
		Pattern: "PATTERN_.*",
	}
	err := patternRef.Compile()
	require.NoError(t, err)

	env.AddEnvFrom([]profile.EnvFromSource{
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
		setupEnv func() profile.Command
		validate func(t *testing.T, result []string)
		name     string
	}{
		{
			name: "base environment with malformed entry",
			setupEnv: func() profile.Command {
				t.Helper()
				baseEnv := []string{"VALID_VAR=value", "MALFORMED_NO_EQUALS", "PATH=/usr/bin", "ANOTHER_VALID=another"}

				return profile.NewCommand(baseEnv)
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
			setupEnv: func() profile.Command {
				t.Helper()
				env := profile.NewCommand([]string{})
				env.AddEnvVar(profile.EnvVar{
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

	env := profile.NewCommand(baseEnv)

	// Set up a pattern that would match TEST_MATCH_.
	callerRef := &profile.CallerRef{
		Pattern: "TEST_MATCH_.*",
	}
	err := callerRef.Compile()
	require.NoError(t, err)

	env.AddEnvFrom([]profile.EnvFromSource{
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

	env := profile.NewCommand(baseEnv)

	// First, add the non-essential variable via envFrom.
	env.AddEnvFrom([]profile.EnvFromSource{
		{
			CallerRef: &profile.CallerRef{
				Name: "CUSTOM_VAR",
			},
		},
	})

	// Then, add an env var that references it via caller reference.
	env.AddEnvVar(profile.EnvVar{
		Name: "COPIED_CUSTOM",
		ValueFrom: &profile.EnvVarSource{
			CallerRef: &profile.CallerRef{
				Name: "CUSTOM_VAR",
			},
		},
	})

	result := env.GetEnv()

	// Both the original and the copied variable should be present.
	assert.Contains(t, result, "CUSTOM_VAR=custom_value")
	assert.Contains(t, result, "COPIED_CUSTOM=custom_value")
}
