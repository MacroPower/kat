package runtimeconfigs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api/v1beta1/runtimeconfigs"
	"github.com/macropower/kat/pkg/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := runtimeconfigs.New()

	assert.NotNil(t, cfg)
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
	assert.Equal(t, "RuntimeConfig", cfg.GetKind())
	assert.NotNil(t, cfg.Command)
}

func TestRuntimeConfig_EnsureDefaults(t *testing.T) {
	t.Parallel()

	cfg := &runtimeconfigs.RuntimeConfig{}

	// Before EnsureDefaults, Command should be nil.
	assert.Nil(t, cfg.Command)

	cfg.EnsureDefaults()

	// After EnsureDefaults, Command should be set.
	assert.NotNil(t, cfg.Command)
}

func TestFind(t *testing.T) {
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
				configPath := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: RuntimeConfig\n"),
					0o600,
				)
				require.NoError(t, err)

				return dir
			},
			want:    ".katrc.yaml",
			wantErr: false,
		},
		"finds config in parent directory": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()
				configPath := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: RuntimeConfig\n"),
					0o600,
				)
				require.NoError(t, err)

				// Create a subdirectory and return it.
				subDir := filepath.Join(dir, "subdir")
				err = os.MkdirAll(subDir, 0o700)
				require.NoError(t, err)

				return subDir
			},
			want:    ".katrc.yaml",
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
				configPath := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: RuntimeConfig\n"),
					0o600,
				)
				require.NoError(t, err)

				// Create a file and return its path.
				filePath := filepath.Join(dir, "test.yaml")
				err = os.WriteFile(filePath, []byte("test"), 0o600)
				require.NoError(t, err)

				return filePath
			},
			want:    ".katrc.yaml",
			wantErr: false,
		},
		"prefers .katrc.yaml over katrc.yaml": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create both files.
				dotKatPath := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(dotKatPath, []byte("dot-kat"), 0o600)
				require.NoError(t, err)

				katPath := filepath.Join(dir, "katrc.yaml")
				err = os.WriteFile(katPath, []byte("kat"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    ".katrc.yaml",
			wantErr: false,
		},
		"finds katrc.yaml if .katrc.yaml not present": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create only katrc.yaml.
				katPath := filepath.Join(dir, "katrc.yaml")
				err := os.WriteFile(katPath, []byte("kat"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    "katrc.yaml",
			wantErr: false,
		},
		"finds .katrc.yml if .yaml not present": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create only .katrc.yml.
				ymlPath := filepath.Join(dir, ".katrc.yml")
				err := os.WriteFile(ymlPath, []byte("yml"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    ".katrc.yml",
			wantErr: false,
		},
		"finds katrc.yml if other options not present": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create only katrc.yml.
				ymlPath := filepath.Join(dir, "katrc.yml")
				err := os.WriteFile(ymlPath, []byte("yml"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    "katrc.yml",
			wantErr: false,
		},
		"prefers .yaml over .yml": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create both .yaml and .yml files.
				yamlPath := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(yamlPath, []byte("yaml"), 0o600)
				require.NoError(t, err)

				ymlPath := filepath.Join(dir, ".katrc.yml")
				err = os.WriteFile(ymlPath, []byte("yml"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    ".katrc.yaml",
			wantErr: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			targetPath := tc.setup(t)

			got, err := runtimeconfigs.Find(targetPath)

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

func TestRuntimeConfigLoader_Load(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		wantErr bool
	}{
		"valid minimal config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
`,
			wantErr: false,
		},
		"valid config with profiles": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
profiles:
  custom:
    command: make
    args: [render]
`,
			wantErr: false,
		},
		"valid config with rules": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
rules:
  - match: 'files.exists(f, pathBase(f) == "Makefile")'
    profile: custom
profiles:
  custom:
    command: make
    args: [render]
`,
			wantErr: false,
		},
		"invalid yaml": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
  invalid: yaml
`,
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pcl := config.NewLoaderFromBytes([]byte(tc.input), runtimeconfigs.New, runtimeconfigs.DefaultValidator)

			cfg, err := pcl.Load()

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
				assert.Equal(t, "RuntimeConfig", cfg.GetKind())
			}
		})
	}
}

func TestRuntimeConfigLoader_Validate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		wantErr bool
	}{
		"valid config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
`,
			wantErr: false,
		},
		"missing apiVersion": {
			input: `kind: RuntimeConfig
`,
			wantErr: true,
		},
		"missing kind": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
`,
			wantErr: true,
		},
		"invalid kind": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`,
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pcl := config.NewLoaderFromBytes([]byte(tc.input), runtimeconfigs.New, runtimeconfigs.DefaultValidator)

			err := pcl.Validate()

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewRuntimeConfigLoaderFromFile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFile func(t *testing.T) string
		wantErr   bool
	}{
		"valid file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				content := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
`
				dir := t.TempDir()
				path := filepath.Join(dir, ".katrc.yaml")
				err := os.WriteFile(path, []byte(content), 0o600)
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
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setupFile(t)

			got, err := config.NewLoaderFromFile(path, runtimeconfigs.New, runtimeconfigs.DefaultValidator)

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

func TestRuntimeConfig_Validate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setup   func() *runtimeconfigs.RuntimeConfig
		wantErr bool
	}{
		"valid config": {
			setup:   runtimeconfigs.New,
			wantErr: false,
		},
		"nil command": {
			setup: func() *runtimeconfigs.RuntimeConfig {
				cfg := &runtimeconfigs.RuntimeConfig{}

				return cfg
			},
			wantErr: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := tc.setup()

			err := cfg.Validate()

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
