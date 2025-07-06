package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/config"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	cfg := config.NewConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.APIVersion)
	assert.Equal(t, "Configuration", cfg.Kind)
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Command)
}

func TestConfig_EnsureDefaults(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "Configuration",
	}

	// Before EnsureDefaults, UI and Kube should be nil.
	assert.Nil(t, cfg.UI)
	assert.Nil(t, cfg.Command)

	cfg.EnsureDefaults()

	// After EnsureDefaults, both should be set.
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Command)
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		apiVersion string
		kind       string
		errMsg     string
		wantErr    bool
	}{
		"valid config": {
			apiVersion: "kat.jacobcolvin.com/v1beta1",
			kind:       "Configuration",
			wantErr:    false,
		},
		"invalid apiVersion": {
			apiVersion: "invalid/v1",
			kind:       "Configuration",
			wantErr:    true,
			errMsg:     "unsupported apiVersion",
		},
		"invalid kind": {
			apiVersion: "kat.jacobcolvin.com/v1beta1",
			kind:       "InvalidKind",
			wantErr:    true,
			errMsg:     "unsupported kind",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				APIVersion: tc.apiVersion,
				Kind:       tc.kind,
			}

			err := cfg.Validate()

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReadConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupFile func(t *testing.T) string
		errMsg    string
		wantErr   bool
	}{
		"valid file": {
			setupFile: func(t *testing.T) string {
				t.Helper()
				content := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`

				return createTempFile(t, content)
			},
			wantErr: false,
		},
		"non-existent file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				return "/non/existent/file.yaml"
			},
			wantErr: true,
			errMsg:  "stat file",
		},
		"directory instead of file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			wantErr: true,
			errMsg:  "path is a directory",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupFile(t)

			data, err := config.ReadConfig(path)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, data)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, data)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlContent string
		errMsg      string
		wantErr     bool
	}{
		"valid config": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
rules:
  - match: 'true'
    profile: test
profiles:
  test:
    command: echo
    args: ["test"]
`,
			wantErr: false,
		},
		"invalid yaml": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
invalid: [unclosed
`,
			wantErr: true,
			errMsg:  "parse YAML",
		},
		"missing required fields": {
			yamlContent: `profiles:
  test:
    command: echo
`,
			wantErr: true,
			errMsg:  "missing properties 'apiVersion', 'kind'",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.LoadConfig([]byte(tc.yamlContent))

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.APIVersion)
				assert.Equal(t, "Configuration", cfg.Kind)
			}
		})
	}
}

func TestConfig_Write(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupPath func(t *testing.T) string
		errMsg    string
		wantErr   bool
	}{
		"new file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml")
			},
			wantErr: false,
		},
		"existing file": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "config.yaml")
				err := os.WriteFile(path, []byte("existing"), 0o600)
				require.NoError(t, err)

				return path
			},
			wantErr: false, // Should not overwrite existing file.
		},
		"directory exists": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				return filepath.Join(dir, "subdir", "config.yaml")
			},
			wantErr: false, // Should create parent directories.
		},
		"path is directory": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			wantErr: true,
			errMsg:  "path is a directory",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := config.NewConfig()
			path := tc.setupPath(t)

			err := cfg.Write(path)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
				// Verify file exists and has content.
				_, err := os.Stat(path)
				require.NoError(t, err)
			}
		})
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupPath func(t *testing.T) string
		errMsg    string
		wantErr   bool
	}{
		"new file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml")
			},
			wantErr: false,
		},
		"existing file": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "config.yaml")
				err := os.WriteFile(path, []byte("existing"), 0o600)
				require.NoError(t, err)

				return path
			},
			wantErr: false, // Should not overwrite existing file.
		},
		"create parent directories": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				return filepath.Join(dir, "nested", "deep", "config.yaml")
			},
			wantErr: false,
		},
		"path is directory": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			wantErr: true,
			errMsg:  "path is a directory",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupPath(t)

			err := config.WriteDefaultConfig(path)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
				// Verify file exists and has content.
				info, err := os.Stat(path)
				require.NoError(t, err)
				assert.True(t, info.Mode().IsRegular())
				assert.Positive(t, info.Size())
			}
		})
	}
}

func TestDefaultConfigYAMLIsValid(t *testing.T) {
	t.Parallel()

	// Write the default config to a temporary file.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "default-config.yaml")

	err := config.WriteDefaultConfig(configPath)
	require.NoError(t, err)

	// Read and load the written config.
	data, err := config.ReadConfig(configPath)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(data)
	require.NoError(t, err)

	// Re-marshal the config to get only public fields.
	cfg.UI.KeyBinds.Common.Help.Keys[0].Code = "ctrl+h" // Hack since "?" doesn't unmarshal correctly in YAMLEq.
	cfgYAML, err := cfg.MarshalYAML()
	require.NoError(t, err)

	defaultCfg := config.NewConfig()
	defaultCfg.UI.KeyBinds.Common.Help.Keys[0].Code = "ctrl+h" // Hack since "?" doesn't unmarshal correctly in YAMLEq.
	defaultCfgYAML, err := defaultCfg.MarshalYAML()
	require.NoError(t, err)

	assert.YAMLEq(t, string(defaultCfgYAML), string(cfgYAML), "Default config should match the loaded config")
}

func TestConfig_MarshalYAML(t *testing.T) {
	t.Parallel()

	cfg := config.NewConfig()

	data, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the marshaled YAML contains expected fields.
	yamlStr := string(data)
	assert.Contains(t, yamlStr, "apiVersion: kat.jacobcolvin.com/v1beta1")
	assert.Contains(t, yamlStr, "kind: Configuration")
}

//nolint:paralleltest // We need to set environment variables, so run tests sequentially.
func TestGetPath(t *testing.T) {
	tests := map[string]struct {
		setupEnv   func(t *testing.T)
		cleanupEnv func(t *testing.T)
		validate   func(t *testing.T, path string)
	}{
		"with XDG_CONFIG_HOME": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("XDG_CONFIG_HOME", "/custom/config")
			},
			validate: func(t *testing.T, path string) {
				t.Helper()
				assert.Equal(t, "/custom/config/kat/config.yaml", path)
			},
		},
		"without XDG_CONFIG_HOME": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("XDG_CONFIG_HOME", "")
			},
			validate: func(t *testing.T, path string) {
				t.Helper()
				// Should fall back to ~/.config/kat/config.yaml or temp dir.
				assert.Contains(t, path, "kat/config.yaml")
			},
		},
	}

	//nolint:paralleltest // We need to set environment variables, so run tests sequentially.
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			path := config.GetPath()
			assert.NotEmpty(t, path)
			assert.True(t, strings.HasSuffix(path, "config.yaml"))

			if tc.validate != nil {
				tc.validate(t, path)
			}
		})
	}
}

func TestDefaultConfigContentValidation(t *testing.T) {
	t.Parallel()

	// Write default config and read it back.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	err := config.WriteDefaultConfig(configPath)
	require.NoError(t, err)

	// Read the actual file content.
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify essential sections exist.
	assert.Contains(t, contentStr, "apiVersion: kat.jacobcolvin.com/v1beta1")
	assert.Contains(t, contentStr, "kind: Configuration")
	assert.Contains(t, contentStr, "rules:")
	assert.Contains(t, contentStr, "profiles:")

	// Verify specific profiles exist.
	assert.Contains(t, contentStr, "ks:")
	assert.Contains(t, contentStr, "helm:")
	assert.Contains(t, contentStr, "yaml:")

	// Verify profile structure.
	assert.Contains(t, contentStr, "command: kustomize")
	assert.Contains(t, contentStr, "command: helm")
	assert.Contains(t, contentStr, "args:")

	// Verify rules structure.
	assert.Contains(t, contentStr, "match:")
	assert.Contains(t, contentStr, "profile:")
}

func TestEmbeddedConfigMatchesSourceFile(t *testing.T) {
	t.Parallel()

	// Read the source config.yaml file.
	sourceConfig, err := os.ReadFile("config.yaml")
	require.NoError(t, err)

	// Write the embedded config to a temp file.
	tempDir := t.TempDir()
	embeddedConfigPath := filepath.Join(tempDir, "embedded-config.yaml")

	err = config.WriteDefaultConfig(embeddedConfigPath)
	require.NoError(t, err)

	// Read the written embedded config.
	embeddedConfig, err := os.ReadFile(embeddedConfigPath)
	require.NoError(t, err)

	// They should be identical.
	assert.Equal(t, string(sourceConfig), string(embeddedConfig))
}

func TestUnmarshalAndValidateDefaultConfig(t *testing.T) {
	t.Parallel()

	// Write the embedded default config to a temporary file.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "default-config.yaml")

	err := config.WriteDefaultConfig(configPath)
	require.NoError(t, err)

	// Read the config data.
	configData, err := config.ReadConfig(configPath)
	require.NoError(t, err)

	// Load and validate the config using the same process as the main application.
	cfg, err := config.LoadConfig(configData)
	require.NoError(t, err, "embedded default config should load without errors")

	// Validate the config structure.
	err = cfg.Validate()
	require.NoError(t, err, "embedded default config should pass validation")

	// Validate the Kube configuration.
	kubeErr := cfg.Command.Validate()
	assert.Nil(t, kubeErr, "embedded default config Kube section should pass validation")

	// Validate the UI configuration key binds.
	err = cfg.UI.KeyBinds.Validate()
	require.NoError(t, err, "embedded default config UI key binds should pass validation")

	// Verify essential config properties.
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.APIVersion)
	assert.Equal(t, "Configuration", cfg.Kind)
	assert.NotNil(t, cfg.Command)
	assert.NotNil(t, cfg.UI)

	// Verify that all expected profiles exist and are valid.
	expectedProfiles := []string{"ks", "helm", "yaml", "ks-helm"}
	for _, profileName := range expectedProfiles {
		profile, exists := cfg.Command.Profiles[profileName]
		assert.True(t, exists, "profile %q should exist in default config", profileName)
		assert.NotEmpty(t, profile.Command.Command, "profile %q should have a command", profileName)
		assert.NotEmpty(t, profile.Command.Args, "profile %q should have args", profileName)
	}

	// Verify that rules exist and can be evaluated.
	assert.NotEmpty(t, cfg.Command.Rules, "default config should have rules")
	assert.Len(t, cfg.Command.Rules, 3, "default config should have exactly 3 rules")

	// Verify that each rule has the required fields.
	for i, rule := range cfg.Command.Rules {
		assert.NotEmpty(t, rule.Match, "rule %d should have a match expression", i)
		assert.NotEmpty(t, rule.Profile, "rule %d should specify a profile", i)
	}

	// Verify that profile references in rules are valid.
	for i, rule := range cfg.Command.Rules {
		_, exists := cfg.Command.Profiles[rule.Profile]
		assert.True(t, exists, "rule %d references profile %q which should exist", i, rule.Profile)
	}
}

func TestDefaultConfigFullPipeline(t *testing.T) {
	t.Parallel()

	// This test simulates the exact same pipeline used in main.go to ensure
	// the embedded default config works correctly in all scenarios.

	// Write the embedded default config.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	err := config.WriteDefaultConfig(configPath)
	require.NoError(t, err)

	// Read the config (simulating config.ReadConfig).
	cfgData, err := config.ReadConfig(configPath)
	require.NoError(t, err)

	// Load the config (simulating config.LoadConfig).
	cfg, err := config.LoadConfig(cfgData)
	require.NoError(t, err)

	// Validate UI KeyBinds (simulating the validation in main.go).
	err = cfg.UI.KeyBinds.Validate()
	require.NoError(t, err)

	// Test that the config can be marshaled back to YAML.
	yamlConfig, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.NotEmpty(t, yamlConfig)

	// Verify the marshaled config can be loaded again (round-trip test).
	cfg2, err := config.LoadConfig(yamlConfig)
	require.NoError(t, err)
	assert.Equal(t, cfg.APIVersion, cfg2.APIVersion)
	assert.Equal(t, cfg.Kind, cfg2.Kind)
	assert.Len(t, cfg2.Command.Profiles, len(cfg.Command.Profiles))
	assert.Len(t, cfg2.Command.Rules, len(cfg.Command.Rules))
}

// createTempFile creates a temporary file with the given content.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}
