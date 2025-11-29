package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api"
)

//nolint:paralleltest // We need to set environment variables, so run tests sequentially.
func TestGetConfigPath(t *testing.T) {
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

			got := api.GetConfigPath("config.yaml")

			assert.NotEmpty(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReadFile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFile func(t *testing.T) string
		wantErr   bool
	}{
		"valid file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(t.TempDir(), "test.yaml")
				err := os.WriteFile(path, []byte("content"), 0o600)
				require.NoError(t, err)

				return path
			},
			wantErr: false,
		},
		"non-existent file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				return "/non/existent/file.yaml"
			},
			wantErr: true,
		},
		"directory instead of file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupFile(t)

			got, err := api.ReadFile(path)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestMarshalYAML(t *testing.T) {
	t.Parallel()

	type testObj struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	obj := testObj{Name: "test", Value: 42}

	data, err := api.MarshalYAML(obj)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "name: test")
	assert.Contains(t, string(data), "value: 42")
}

func TestWriteIfNotExists(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupPath func(t *testing.T) string
		errMsg    string
		wantErr   bool
	}{
		"new file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "new.yaml")
			},
			wantErr: false,
		},
		"existing file": {
			setupPath: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(t.TempDir(), "existing.yaml")
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

				return filepath.Join(dir, "nested", "deep", "file.yaml")
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

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupPath(t)

			err := api.WriteIfNotExists(path, []byte("new content"))

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)

				_, err := os.Stat(path)
				require.NoError(t, err)
			}
		})
	}
}

func TestWriteDefaultFile(t *testing.T) {
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
		"existing file without force": {
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
		"creates parent directories": {
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
		"force creates backup": {
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

			err = api.WriteDefaultFile(path, []byte("default content"), tc.force, "test")

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)

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

func TestFindConfigFile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setup   func(t *testing.T) string
		want    string
		wantErr bool
	}{
		"finds config in current directory": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()
				configPath := filepath.Join(dir, ".kat.yaml")
				err := os.WriteFile(configPath, []byte("content"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    ".kat.yaml",
			wantErr: false,
		},
		"finds config in parent directory": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()
				configPath := filepath.Join(dir, ".kat.yaml")
				err := os.WriteFile(configPath, []byte("content"), 0o600)
				require.NoError(t, err)

				// Create a subdirectory and return it.
				subDir := filepath.Join(dir, "subdir")
				err = os.MkdirAll(subDir, 0o700)
				require.NoError(t, err)

				return subDir
			},
			want:    ".kat.yaml",
			wantErr: false,
		},
		"returns empty when not found": {
			setup: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			want:    "",
			wantErr: false,
		},
		"handles file path input": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()
				configPath := filepath.Join(dir, ".kat.yaml")
				err := os.WriteFile(configPath, []byte("content"), 0o600)
				require.NoError(t, err)

				// Create a file and return its path.
				filePath := filepath.Join(dir, "test.yaml")
				err = os.WriteFile(filePath, []byte("test"), 0o600)
				require.NoError(t, err)

				return filePath
			},
			want:    ".kat.yaml",
			wantErr: false,
		},
	}

	fileNames := []string{".kat.yaml", "kat.yaml"}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			targetPath := tc.setup(t)

			got, err := api.FindConfigFile(targetPath, fileNames)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.want == "" {
					assert.Empty(t, got)
				} else {
					assert.Contains(t, got, tc.want)
				}
			}
		})
	}
}
