package configs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api/v1beta1/configs"
	"github.com/macropower/kat/pkg/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := configs.New()

	assert.NotNil(t, cfg)
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
	assert.Equal(t, "Configuration", cfg.GetKind())
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Command)
}

func TestConfig_EnsureDefaults(t *testing.T) {
	t.Parallel()

	cfg := &configs.Config{}

	// Before EnsureDefaults, UI and Command should be nil.
	assert.Nil(t, cfg.UI)
	assert.Nil(t, cfg.Command)

	cfg.EnsureDefaults()

	// After EnsureDefaults, both should be set.
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Command)
}

func TestConfig_Write(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
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
		"creates parent directories": {
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

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := configs.New()
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

func TestWriteDefault(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
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
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupPath(t)

			// Record if the file existed before to check backup behavior.
			var originalContent []byte

			info, err := os.Stat(path)
			if err == nil && info.Mode().IsRegular() {
				originalContent, err = os.ReadFile(path)
				require.NoError(t, err)
			}

			err = configs.WriteDefault(path, tc.force)

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

				// If force=true and original content existed, verify backup was created.
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
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			got := configs.GetPath()

			assert.NotEmpty(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultConfigYAMLIsValid(t *testing.T) {
	t.Parallel()

	// Write the default config to a temporary file.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "default-config.yaml")

	err := configs.WriteDefault(configPath, false)
	require.NoError(t, err)

	// Load the written config using the Loader API.
	cl, err := config.NewLoaderFromFile(configPath, configs.New, configs.DefaultValidator)
	require.NoError(t, err)

	cfg, err := cl.Load()
	require.NoError(t, err)

	// Re-marshal the config to get only public fields.
	cfg.UI.KeyBinds.Common.Help.Keys[0].Code = "ctrl+h" // Hack since "?" doesn't unmarshal correctly in YAMLEq.
	cfgYAML, err := cfg.MarshalYAML()
	require.NoError(t, err)

	defaultCfg := configs.New()
	defaultCfg.UI.KeyBinds.Common.Help.Keys[0].Code = "ctrl+h" // Hack since "?" doesn't unmarshal correctly in YAMLEq.
	defaultCfgYAML, err := defaultCfg.MarshalYAML()
	require.NoError(t, err)

	assert.YAMLEq(t, string(defaultCfgYAML), string(cfgYAML), "Default config should match the loaded config")
}

func TestConfig_MarshalYAML(t *testing.T) {
	t.Parallel()

	cfg := configs.New()

	data, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the marshaled YAML contains expected fields.
	yamlStr := string(data)
	assert.Contains(t, yamlStr, "apiVersion: kat.jacobcolvin.com/v1beta1")
	assert.Contains(t, yamlStr, "kind: Configuration")
}

func TestEmbeddedConfigMatchesSourceFile(t *testing.T) {
	t.Parallel()

	// Read the source config.yaml file.
	sourceConfig, err := os.ReadFile("config.yaml")
	require.NoError(t, err)

	// Write the embedded config to a temp file.
	tempDir := t.TempDir()
	embeddedConfigPath := filepath.Join(tempDir, "embedded-config.yaml")

	err = configs.WriteDefault(embeddedConfigPath, false)
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

	err := configs.WriteDefault(configPath, false)
	require.NoError(t, err)

	// Load and validate the config using the same process as the main application.
	cl, err := config.NewLoaderFromFile(configPath, configs.New, configs.DefaultValidator)
	require.NoError(t, err)

	cfg, err := cl.Load()
	require.NoError(t, err, "embedded default config should load without errors")

	// Validate the Command configuration.
	cmdErr := cfg.Command.Validate()
	require.NoError(t, cmdErr, "embedded default config Command section should pass validation")

	// Validate the UI configuration key binds.
	err = cfg.UI.KeyBinds.Validate()
	require.NoError(t, err, "embedded default config UI key binds should pass validation")

	// Verify essential config properties.
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
	assert.Equal(t, "Configuration", cfg.GetKind())
	assert.NotNil(t, cfg.Command)
	assert.NotNil(t, cfg.UI)

	// Verify that profiles exist.
	assert.NotEmpty(t, cfg.Command.Profiles, "default config should have profiles")
	// Verify that all profiles are valid.
	for profileName, p := range cfg.Command.Profiles {
		assert.NotEmpty(t, p.Command.Command, "profile %q should have a command", profileName)
	}

	// Verify that rules exist.
	assert.NotEmpty(t, cfg.Command.Rules, "default config should have rules")

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

	err := configs.WriteDefault(configPath, false)
	require.NoError(t, err)

	// Load the config using the Loader API.
	cl, err := config.NewLoaderFromFile(configPath, configs.New, configs.DefaultValidator)
	require.NoError(t, err)

	cfg, err := cl.Load()
	require.NoError(t, err)

	// Validate UI KeyBinds (simulating the validation in main.go).
	err = cfg.UI.KeyBinds.Validate()
	require.NoError(t, err)

	// Test that the config can be marshaled back to YAML.
	yamlConfig, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.NotEmpty(t, yamlConfig)

	// Verify the marshaled config can be loaded again (round-trip test).
	cl2 := config.NewLoaderFromBytes(yamlConfig, configs.New, configs.DefaultValidator)
	cfg2, err := cl2.Load()
	require.NoError(t, err)
	assert.Equal(t, cfg.GetAPIVersion(), cfg2.GetAPIVersion())
	assert.Equal(t, cfg.GetKind(), cfg2.GetKind())
	assert.Len(t, cfg2.Command.Profiles, len(cfg.Command.Profiles))
	assert.Len(t, cfg2.Command.Rules, len(cfg.Command.Rules))
}
