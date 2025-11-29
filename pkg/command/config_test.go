package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

// TestConfigError_Error tests that ConfigError properly formats error messages.
func TestConfigError_Error_Format(t *testing.T) {
	t.Parallel()

	// We'll test this indirectly through validation errors that create real ConfigErrors
	// since creating a yaml.Path requires complex setup

	// Create an invalid config that will trigger a ConfigError
	config := &command.Config{
		Profiles: map[string]*profile.Profile{
			"test": profile.MustNew("echo", profile.WithArgs("test")),
		},
		Rules: []*rule.Rule{
			rule.MustNew("nonexistent", `true`), // This will cause validation to fail
		},
	}

	err := config.Validate()
	require.Error(t, err)

	// The error should contain validation information about the missing profile
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestConfig_EnsureDefaults(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config  *command.Config
		checkFn func(*testing.T, *command.Config)
	}{
		"nil profiles and rules": {
			config: &command.Config{},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.NotNil(t, c.Profiles)
				assert.NotNil(t, c.Rules)
				// Should have default profiles
				assert.Contains(t, c.Profiles, "ks")
				assert.Contains(t, c.Profiles, "helm")
				assert.Contains(t, c.Profiles, "yaml")
				// Should have default rules
				assert.Len(t, c.Rules, 3)
			},
		},
		"existing profiles nil rules": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"custom": profile.MustNew("echo", profile.WithArgs("test")),
				},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Profiles, 1)
				assert.Contains(t, c.Profiles, "custom")
				assert.NotNil(t, c.Rules)
				assert.Len(t, c.Rules, 3) // Should get default rules
			},
		},
		"nil profiles existing rules": {
			config: &command.Config{
				Rules: []*rule.Rule{
					rule.MustNew("custom", `true`),
				},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.NotNil(t, c.Profiles)
				assert.Len(t, c.Rules, 1)
				assert.Equal(t, "custom", c.Rules[0].Profile)
				// Should get default profiles
				assert.Contains(t, c.Profiles, "ks")
			},
		},
		"both exist": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"custom": profile.MustNew("echo", profile.WithArgs("test")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("custom", `true`),
				},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Profiles, 1)
				assert.Len(t, c.Rules, 1)
				// Should not be modified
				assert.Contains(t, c.Profiles, "custom")
				assert.Equal(t, "custom", c.Rules[0].Profile)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.config.EnsureDefaults()
			tc.checkFn(t, tc.config)
		})
	}
}

func TestConfig_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config      *command.Config
		errorPath   string
		expectError bool
	}{
		"invalid profile source": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"invalid": {
						Source: "invalid CEL expression [[[",
					},
				},
				Rules: []*rule.Rule{},
			},
			expectError: true,
			errorPath:   "profiles.invalid.source",
		},
		"invalid rule match": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"test": profile.MustNew("echo", profile.WithArgs("test")),
				},
				Rules: []*rule.Rule{
					{
						Profile: "test",
						Match:   "invalid CEL expression [[[",
					},
				},
			},
			expectError: true,
			errorPath:   "rules[0].match",
		},
		"rule references non-existent profile": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"test": profile.MustNew("echo", profile.WithArgs("test")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("nonexistent", `true`),
				},
			},
			expectError: true,
			errorPath:   "rules[0].profile",
		},
		"valid config": {
			config: &command.Config{
				Profiles: map[string]*profile.Profile{
					"test": profile.MustNew("echo", profile.WithArgs("test")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("test", `true`),
				},
			},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()

			if tc.expectError {
				require.Error(t, err)
				if tc.errorPath != "" {
					assert.Contains(t, err.Error(), tc.errorPath)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	t.Parallel()

	// Test that NewConfig returns a valid config with defaults
	config := command.NewConfig()
	require.NotNil(t, config)

	// Should have default profiles
	assert.Contains(t, config.Profiles, "ks")
	assert.Contains(t, config.Profiles, "helm")
	assert.Contains(t, config.Profiles, "yaml")

	// Should have default rules
	assert.Len(t, config.Rules, 3)

	// Default config should validate successfully
	err := config.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupConfig func() *command.Config
		errorPath   string
		expectError bool
	}{
		"empty config": {
			setupConfig: func() *command.Config {
				return &command.Config{
					Profiles: map[string]*profile.Profile{},
					Rules:    []*rule.Rule{},
				}
			},
			expectError: false, // Empty config is valid
		},
		"profile with valid configuration": {
			setupConfig: func() *command.Config {
				return &command.Config{
					Profiles: map[string]*profile.Profile{
						"test": profile.MustNew("echo", profile.WithArgs("test")),
					},
					Rules: []*rule.Rule{
						rule.MustNew("test", `true`),
					},
				}
			},
			expectError: false, // This should be valid
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := tc.setupConfig()
			configErr := config.Validate()

			if tc.expectError {
				require.Error(t, configErr)
				if tc.errorPath != "" {
					assert.Contains(t, configErr.Error(), tc.errorPath)
				}
			} else {
				assert.NoError(t, configErr)
			}
		})
	}
}

func TestConfig_Merge(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		global  *command.Config
		project *command.Config
		checkFn func(*testing.T, *command.Config)
	}{
		"nil project config": {
			global: &command.Config{
				Profiles: map[string]*profile.Profile{
					"global": profile.MustNew("echo", profile.WithArgs("global")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("global", `true`),
				},
			},
			project: nil,
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Profiles, 1)
				assert.Contains(t, c.Profiles, "global")
				assert.Len(t, c.Rules, 1)
			},
		},
		"project profiles override global": {
			global: &command.Config{
				Profiles: map[string]*profile.Profile{
					"shared": profile.MustNew("echo", profile.WithArgs("global")),
					"global": profile.MustNew("echo", profile.WithArgs("global-only")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("shared", `true`),
				},
			},
			project: &command.Config{
				Profiles: map[string]*profile.Profile{
					"shared":  profile.MustNew("echo", profile.WithArgs("project")),
					"project": profile.MustNew("echo", profile.WithArgs("project-only")),
				},
				Rules: []*rule.Rule{},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Profiles, 3) // shared, global, project
				assert.Contains(t, c.Profiles, "shared")
				assert.Contains(t, c.Profiles, "global")
				assert.Contains(t, c.Profiles, "project")
				// The shared profile should be from project (override)
				assert.Equal(t, []string{"project"}, c.Profiles["shared"].Command.Args)
			},
		},
		"project rules prepended": {
			global: &command.Config{
				Profiles: map[string]*profile.Profile{
					"global":  profile.MustNew("echo", profile.WithArgs("global")),
					"project": profile.MustNew("echo", profile.WithArgs("project")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("global", `true`),
				},
			},
			project: &command.Config{
				Profiles: map[string]*profile.Profile{},
				Rules: []*rule.Rule{
					rule.MustNew("project", `true`),
				},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Rules, 2)
				// Project rule should be first (prepended)
				assert.Equal(t, "project", c.Rules[0].Profile)
				assert.Equal(t, "global", c.Rules[1].Profile)
			},
		},
		"empty project config": {
			global: &command.Config{
				Profiles: map[string]*profile.Profile{
					"global": profile.MustNew("echo", profile.WithArgs("global")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("global", `true`),
				},
			},
			project: &command.Config{
				Profiles: map[string]*profile.Profile{},
				Rules:    []*rule.Rule{},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.Len(t, c.Profiles, 1)
				assert.Len(t, c.Rules, 1)
			},
		},
		"global nil profiles": {
			global: &command.Config{
				Profiles: nil,
				Rules:    []*rule.Rule{},
			},
			project: &command.Config{
				Profiles: map[string]*profile.Profile{
					"project": profile.MustNew("echo", profile.WithArgs("project")),
				},
				Rules: []*rule.Rule{
					rule.MustNew("project", `true`),
				},
			},
			checkFn: func(t *testing.T, c *command.Config) {
				t.Helper()
				assert.NotNil(t, c.Profiles)
				assert.Len(t, c.Profiles, 1)
				assert.Contains(t, c.Profiles, "project")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.global.Merge(tc.project)
			tc.checkFn(t, tc.global)
		})
	}
}
