package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api/v1beta1/configs"
	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/ui/theme"
)

func TestNewLoaderFromFile(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFile func(t *testing.T) string
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

			got, err := config.NewLoaderFromFile(path, configs.New, configs.DefaultValidator)

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

func TestNewLoaderFromBytes(t *testing.T) {
	t.Parallel()

	input := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
rules:
  - match: 'true'
    profile: test
profiles:
  test:
    command: echo
    args: ["test"]
`

	cl := config.NewLoaderFromBytes([]byte(input), configs.New, configs.DefaultValidator)
	require.NotNil(t, cl)

	err := cl.Validate()
	require.NoError(t, err)

	cfg, err := cl.Load()
	require.NoError(t, err)
	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", cfg.GetAPIVersion())
	assert.Equal(t, "Configuration", cfg.GetKind())
}

func TestLoader_Validate(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		errMsg  string
		wantErr bool
	}{
		"valid config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
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
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
invalid: [unclosed
`,
			wantErr: true,
			errMsg:  "sequence end token ']' not found",
		},
		"missing required fields": {
			input: `profiles:
  test:
    command: echo
`,
			wantErr: true,
			errMsg:  "missing properties 'apiVersion', 'kind'",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cl := config.NewLoaderFromBytes([]byte(tc.input), configs.New, configs.DefaultValidator)

			err := cl.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoader_Load(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input   string
		errMsg  string
		wantErr bool
	}{
		"valid config": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
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
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
invalid: [unclosed
`,
			wantErr: true,
			errMsg:  "sequence end token ']' not found",
		},
		"missing required fields still loads": {
			// Load() only parses YAML, it doesn't validate schema.
			// Use Validate() to check schema requirements.
			input: `profiles:
  test:
    command: echo
`,
			wantErr: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cl := config.NewLoaderFromBytes([]byte(tc.input), configs.New, configs.DefaultValidator)

			cfg, err := cl.Load()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestLoader_GetTheme(t *testing.T) {
	t.Parallel()

	input := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`

	cl := config.NewLoaderFromBytes([]byte(input), configs.New, configs.DefaultValidator)
	require.NotNil(t, cl)

	got := cl.GetTheme()
	require.NotNil(t, got)
	assert.Equal(t, theme.Default.ChromaStyle.Name, got.ChromaStyle.Name)
}

func TestLoader_WithThemeFromData(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		want  *theme.Theme
		input string
	}{
		"valid config with quoted theme": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  theme: "github"`,
			want: theme.New("github"),
		},
		"valid config with single quoted theme": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  theme: 'monokai'`,
			want: theme.New("monokai"),
		},
		"valid config with unquoted theme": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  theme: dracula`,
			want: theme.New("dracula"),
		},
		"config with ui section but no theme": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  someOtherField: value`,
			want: theme.Default,
		},
		"config without ui section": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo`,
			want: theme.Default,
		},
		"malformed yaml with regex fallback": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  theme: "onedark"
  invalid: [unclosed`,
			want: theme.New("onedark"),
		},
		"regex fallback with indented theme": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  someField: value
  theme: "github-dark"
  anotherField: value`,
			want: theme.New("github-dark"),
		},
		"regex fallback with comments": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
ui:
  # Some comment
  theme: "solarized" # inline comment
  otherField: value`,
			want: theme.New("solarized"),
		},
		"empty config": {
			input: "",
			want:  theme.Default,
		},
		"completely invalid yaml": {
			input: `this is not yaml at all!`,
			want:  theme.Default,
		},
		"config with theme in wrong section": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    theme: "shouldnotbefound"`,
			want: theme.Default,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NotNil(t, tc.want)
			require.NotNil(t, tc.want.ChromaStyle)

			cl := config.NewLoaderFromBytes(
				[]byte(tc.input), configs.New, configs.DefaultValidator, config.WithThemeFromData(),
			)
			require.NotNil(t, cl)

			got := cl.GetTheme()
			require.NotNil(t, got)
			require.NotNil(t, got.ChromaStyle)

			assert.Equal(t, tc.want.ChromaStyle.Name, got.ChromaStyle.Name)
		})
	}
}

func TestLoader_WithValidator(t *testing.T) {
	t.Parallel()

	input := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`

	// Test with nil validator (no validation).
	cl := config.NewLoaderFromBytes([]byte(input), configs.New, nil, config.WithValidator(nil))
	require.NotNil(t, cl)

	err := cl.Validate()
	require.NoError(t, err)
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

func TestLoader_LoadCallsEnsureDefaults(t *testing.T) {
	t.Parallel()

	// Config with only apiVersion and kind - UI and Command should be nil before EnsureDefaults.
	input := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`

	cl := config.NewLoaderFromBytes([]byte(input), configs.New, configs.DefaultValidator)

	cfg, err := cl.Load()
	require.NoError(t, err)

	// After Load(), EnsureDefaults should have been called, so UI and Command should not be nil.
	assert.NotNil(t, cfg.UI, "EnsureDefaults should initialize UI")
	assert.NotNil(t, cfg.Command, "EnsureDefaults should initialize Command")
}

func TestLoader_RoundTrip(t *testing.T) {
	t.Parallel()

	// Write the embedded default config.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	err := configs.WriteDefault(configPath, false)
	require.NoError(t, err)

	// Load the config.
	cl, err := config.NewLoaderFromFile(configPath, configs.New, configs.DefaultValidator)
	require.NoError(t, err)

	cfg, err := cl.Load()
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
