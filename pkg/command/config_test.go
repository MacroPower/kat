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
	profiles := map[string]*profile.Profile{
		"test": profile.MustNew("echo", profile.WithArgs("test")),
	}
	rules := []*rule.Rule{
		rule.MustNew("nonexistent", `true`), // This will cause validation to fail
	}

	_, err := command.NewConfig(profiles, rules)
	require.Error(t, err)

	// The error should contain validation information
	assert.Contains(t, err.Error(), "validate config")
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

func TestNewConfig_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		profiles    map[string]*profile.Profile
		errorPath   string
		rules       []*rule.Rule
		expectError bool
	}{
		"invalid profile source": {
			profiles: map[string]*profile.Profile{
				"invalid": {
					Source: "invalid CEL expression [[[",
				},
			},
			rules:       []*rule.Rule{},
			expectError: true,
			errorPath:   "profiles.invalid.source",
		},
		"invalid rule match": {
			profiles: map[string]*profile.Profile{
				"test": profile.MustNew("echo", profile.WithArgs("test")),
			},
			rules: []*rule.Rule{
				{
					Profile: "test",
					Match:   "invalid CEL expression [[[",
				},
			},
			expectError: true,
			errorPath:   "rules[0].match",
		},
		"rule references non-existent profile": {
			profiles: map[string]*profile.Profile{
				"test": profile.MustNew("echo", profile.WithArgs("test")),
			},
			rules: []*rule.Rule{
				rule.MustNew("nonexistent", `true`),
			},
			expectError: true,
			errorPath:   "rules[0].profile",
		},
		"valid config": {
			profiles: map[string]*profile.Profile{
				"test": profile.MustNew("echo", profile.WithArgs("test")),
			},
			rules: []*rule.Rule{
				rule.MustNew("test", `true`),
			},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config, err := command.NewConfig(tc.profiles, tc.rules)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "validate config")
				if tc.errorPath != "" {
					assert.Contains(t, err.Error(), tc.errorPath)
				}

				assert.Nil(t, config)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestMustNewConfig(t *testing.T) {
	t.Parallel()

	// Test successful creation
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		profiles := map[string]*profile.Profile{
			"test": profile.MustNew("echo", profile.WithArgs("test")),
		}
		rules := []*rule.Rule{
			rule.MustNew("test", `true`),
		}

		config := command.MustNewConfig(profiles, rules)
		assert.NotNil(t, config)
		assert.Equal(t, profiles, config.Profiles)
		assert.Equal(t, rules, config.Rules)
	})

	// Test panic on invalid config
	t.Run("panic on invalid", func(t *testing.T) {
		t.Parallel()

		profiles := map[string]*profile.Profile{
			"test": profile.MustNew("echo", profile.WithArgs("test")),
		}
		rules := []*rule.Rule{
			rule.MustNew("nonexistent", `true`),
		}

		assert.Panics(t, func() {
			command.MustNewConfig(profiles, rules)
		})
	})
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
				require.NotNil(t, configErr)
				if tc.errorPath != "" {
					assert.Contains(t, configErr.Error(), tc.errorPath)
				}
			} else {
				assert.Nil(t, configErr)
			}
		})
	}
}
