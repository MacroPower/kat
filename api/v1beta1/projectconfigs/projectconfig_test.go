package projectconfigs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api/v1beta1/projectconfigs"
	"github.com/macropower/kat/pkg/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := projectconfigs.New()

	assert.NotNil(t, cfg)
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
	assert.Equal(t, "ProjectConfig", cfg.GetKind())
	assert.NotNil(t, cfg.Command)
}

func TestProjectConfig_EnsureDefaults(t *testing.T) {
	t.Parallel()

	cfg := &projectconfigs.ProjectConfig{}

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
				configPath := filepath.Join(dir, ".kat.yaml")
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: ProjectConfig\n"),
					0o600,
				)
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
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: ProjectConfig\n"),
					0o600,
				)
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
				err := os.WriteFile(
					configPath,
					[]byte("apiVersion: kat.jacobcolvin.com/v1beta1\nkind: ProjectConfig\n"),
					0o600,
				)
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
		"prefers .kat.yaml over kat.yaml": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create both files.
				dotKatPath := filepath.Join(dir, ".kat.yaml")
				err := os.WriteFile(dotKatPath, []byte("dot-kat"), 0o600)
				require.NoError(t, err)

				katPath := filepath.Join(dir, "kat.yaml")
				err = os.WriteFile(katPath, []byte("kat"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    ".kat.yaml",
			wantErr: false,
		},
		"finds kat.yaml if .kat.yaml not present": {
			setup: func(t *testing.T) string {
				t.Helper()

				dir := t.TempDir()

				// Create only kat.yaml.
				katPath := filepath.Join(dir, "kat.yaml")
				err := os.WriteFile(katPath, []byte("kat"), 0o600)
				require.NoError(t, err)

				return dir
			},
			want:    "kat.yaml",
			wantErr: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			targetPath := tc.setup(t)

			got, err := projectconfigs.Find(targetPath)

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

func TestProjectConfigLoader_Load(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		wantErr bool
	}{
		"valid minimal config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfig
`,
			wantErr: false,
		},
		"valid config with profiles": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfig
profiles:
  custom:
    command: make
    args: [render]
`,
			wantErr: false,
		},
		"valid config with rules": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfig
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
kind: ProjectConfig
  invalid: yaml
`,
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pcl := config.NewLoaderFromBytes([]byte(tc.input), projectconfigs.New, projectconfigs.DefaultValidator)

			cfg, err := pcl.Load()

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
				assert.Equal(t, "ProjectConfig", cfg.GetKind())
			}
		})
	}
}

func TestProjectConfigLoader_Validate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		wantErr bool
	}{
		"valid config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfig
`,
			wantErr: false,
		},
		"missing apiVersion": {
			input: `kind: ProjectConfig
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

			pcl := config.NewLoaderFromBytes([]byte(tc.input), projectconfigs.New, projectconfigs.DefaultValidator)

			err := pcl.Validate()

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewProjectConfigLoaderFromFile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFile func(t *testing.T) string
		wantErr   bool
	}{
		"valid file": {
			setupFile: func(t *testing.T) string {
				t.Helper()

				content := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfig
`
				dir := t.TempDir()
				path := filepath.Join(dir, ".kat.yaml")
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

			got, err := config.NewLoaderFromFile(path, projectconfigs.New, projectconfigs.DefaultValidator)

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

func TestProjectConfig_Validate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setup   func() *projectconfigs.ProjectConfig
		wantErr bool
	}{
		"valid config": {
			setup:   projectconfigs.New,
			wantErr: false,
		},
		"nil command": {
			setup: func() *projectconfigs.ProjectConfig {
				cfg := &projectconfigs.ProjectConfig{}

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
