package config_test

import (
	"os"
	"path/filepath"
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
			errMsg:  "[3:9] sequence end token ']' not found",
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
		force     bool
		wantErr   bool
	}{
		"new file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml")
			},
			force:   false,
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
			force:   false,
			wantErr: false, // Should not overwrite existing file.
		},
		"create parent directories": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				return filepath.Join(dir, "nested", "deep", "config.yaml")
			},
			force:   false,
			wantErr: false,
		},
		"path is directory": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			force:   false,
			wantErr: true,
			errMsg:  "path is a directory",
		},
		"force new file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml")
			},
			force:   true,
			wantErr: false,
		},
		"force existing file creates backup": {
			setupPath: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "config.yaml")
				err := os.WriteFile(path, []byte("existing content"), 0o600)
				require.NoError(t, err)

				return path
			},
			force:   true,
			wantErr: false,
		},
		"force with path is directory": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			force:   true,
			wantErr: true,
			errMsg:  "path is a directory",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupPath(t)

			// Record if the file existed before to check backup behavior
			var originalContent []byte

			info, err := os.Stat(path)
			if err == nil && info.Mode().IsRegular() {
				originalContent, err = os.ReadFile(path)
				require.NoError(t, err)
			}

			err = config.WriteDefaultConfig(path, tc.force)

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

				// If force=true and original content existed, verify backup was created
				if tc.force && len(originalContent) > 0 {
					dir := filepath.Dir(path)
					entries, err := os.ReadDir(dir)
					require.NoError(t, err)

					backupFound := false
					for _, entry := range entries {
						if filepath.Ext(entry.Name()) != ".old" {
							continue
						}

						backupPath := filepath.Join(dir, entry.Name())
						backupContent, err := os.ReadFile(backupPath)
						require.NoError(t, err)
						assert.Equal(t, originalContent, backupContent, "backup should contain original content")

						backupFound = true

						break
					}

					assert.True(t, backupFound, "backup file should be created when force=true and file exists")
				}
			}
		})
	}
}

func TestDefaultConfigYAMLIsValid(t *testing.T) {
	t.Parallel()

	// Write the default config to a temporary file.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "default-config.yaml")

	err := config.WriteDefaultConfig(configPath, false)
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
	tcs := map[string]struct {
		setupEnv func(t *testing.T)
		want     string
	}{
		"XDG_CONFIG_HOME is set and not empty": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("XDG_CONFIG_HOME", "/custom/config")
			},
			want: "/custom/config/kat/config.yaml",
		},
		"XDG_CONFIG_HOME is empty and HOME is set": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "/test/home")
			},
			want: "/test/home/.config/kat/config.yaml",
		},
		"XDG_CONFIG_HOME is not set and HOME is set": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				err := os.Unsetenv("XDG_CONFIG_HOME")
				require.NoError(t, err)
				t.Setenv("HOME", "/test/home")
			},
			want: "/test/home/.config/kat/config.yaml",
		},
		"XDG_CONFIG_HOME is empty and HOME is empty": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "")
			},
			want: filepath.Join(os.TempDir(), "kat", "config.yaml"), //nolint:usetesting // Needs to equal host.
		},
		"XDG_CONFIG_HOME is not set and HOME is empty": {
			setupEnv: func(t *testing.T) {
				t.Helper()
				err := os.Unsetenv("XDG_CONFIG_HOME")
				require.NoError(t, err)
				t.Setenv("HOME", "")
			},
			want: filepath.Join(os.TempDir(), "kat", "config.yaml"), //nolint:usetesting // Needs to equal host.
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			got := config.GetPath()

			assert.NotEmpty(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultConfigContentValidation(t *testing.T) {
	t.Parallel()

	// Write default config and read it back.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	err := config.WriteDefaultConfig(configPath, false)
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

	err = config.WriteDefaultConfig(embeddedConfigPath, false)
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

	err := config.WriteDefaultConfig(configPath, false)
	require.NoError(t, err)

	// Read the config data.
	configData, err := config.ReadConfig(configPath)
	require.NoError(t, err)

	// Load and validate the config using the same process as the main application.
	cfg, err := config.LoadConfig(configData)
	require.NoError(t, err, "embedded default config should load without errors")

	// Validate the Kube configuration.
	kubeErr := cfg.Command.Validate()
	require.NoError(t, kubeErr, "embedded default config Kube section should pass validation")

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

	err := config.WriteDefaultConfig(configPath, false)
	require.NoError(t, err)

	// Read the config.
	cfgData, err := config.ReadConfig(configPath)
	require.NoError(t, err)

	// Load the config.
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
