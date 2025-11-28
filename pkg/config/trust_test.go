package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/config"
)

func TestConfig_IsTrusted(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		checkPath    string
		trustedPaths []string
		want         bool
	}{
		"trusted path returns true": {
			trustedPaths: []string{"/path/to/project"},
			checkPath:    "/path/to/project",
			want:         true,
		},
		"untrusted path returns false": {
			trustedPaths: []string{"/path/to/project"},
			checkPath:    "/path/to/other",
			want:         false,
		},
		"empty trust list returns false": {
			trustedPaths: []string{},
			checkPath:    "/path/to/project",
			want:         false,
		},
		"handles trailing slashes": {
			trustedPaths: []string{"/path/to/project/"},
			checkPath:    "/path/to/project",
			want:         true,
		},
		"nil projects config returns false": {
			trustedPaths: nil,
			checkPath:    "/path/to/project",
			want:         false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := config.NewConfig()

			if tc.trustedPaths == nil {
				cfg.Projects = nil
			} else {
				for _, path := range tc.trustedPaths {
					cfg.Projects.Trust = append(cfg.Projects.Trust, &config.TrustedProject{Path: path})
				}
			}

			got := cfg.IsTrusted(tc.checkPath)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConfig_TrustProject(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		addPath      string
		wantContains string
		initialPaths []string
		wantCount    int
	}{
		"adds new path": {
			initialPaths: []string{},
			addPath:      "/path/to/project",
			wantCount:    1,
			wantContains: "/path/to/project",
		},
		"deduplicates existing path": {
			initialPaths: []string{"/path/to/project"},
			addPath:      "/path/to/project",
			wantCount:    1,
			wantContains: "/path/to/project",
		},
		"adds different path": {
			initialPaths: []string{"/path/to/project1"},
			addPath:      "/path/to/project2",
			wantCount:    2,
			wantContains: "/path/to/project2",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a temp config file.
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")

			cfg := config.NewConfig()

			// Set up initial paths.
			for _, path := range tc.initialPaths {
				cfg.Projects.Trust = append(cfg.Projects.Trust, &config.TrustedProject{Path: path})
			}

			// Write initial config.
			b, err := cfg.MarshalYAML()
			require.NoError(t, err)

			err = os.WriteFile(configPath, b, 0o600)
			require.NoError(t, err)

			// Trust the new project.
			err = cfg.TrustProject(tc.addPath, configPath)
			require.NoError(t, err)

			// Verify in-memory state.
			assert.Len(t, cfg.Projects.Trust, tc.wantCount)

			found := false
			for _, tp := range cfg.Projects.Trust {
				if filepath.Clean(tp.Path) == filepath.Clean(tc.wantContains) {
					found = true

					break
				}
			}

			assert.True(t, found, "expected path %q to be in trust list", tc.wantContains)

			// Verify persisted state by reloading.
			cl, err := config.NewConfigLoaderFromFile(configPath)
			require.NoError(t, err)

			reloaded, err := cl.Load()
			require.NoError(t, err)
			assert.Len(t, reloaded.Projects.Trust, tc.wantCount)
		})
	}
}

func TestConfig_TrustProject_NilProjects(t *testing.T) {
	t.Parallel()

	// Create a temp config file.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	cfg := config.NewConfig()

	// Write initial config.
	b, err := cfg.MarshalYAML()
	require.NoError(t, err)

	err = os.WriteFile(configPath, b, 0o600)
	require.NoError(t, err)

	// Set projects to nil after writing.
	cfg.Projects = nil

	// Trust a project.
	err = cfg.TrustProject("/path/to/project", configPath)
	require.NoError(t, err)

	assert.NotNil(t, cfg.Projects)
	assert.Len(t, cfg.Projects.Trust, 1)
}

func TestConfig_TrustProject_PreservesComments(t *testing.T) {
	t.Parallel()

	// Create a temp config file with comments.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	initialConfig := `# This is a top-level comment
apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
# UI configuration section
ui:
  # Theme setting
  theme: dracula
`

	err := os.WriteFile(configPath, []byte(initialConfig), 0o600)
	require.NoError(t, err)

	// Load and trust a project.
	cl, err := config.NewConfigLoaderFromFile(configPath)
	require.NoError(t, err)

	cfg, err := cl.Load()
	require.NoError(t, err)

	err = cfg.TrustProject("/path/to/project", configPath)
	require.NoError(t, err)

	// Read the file back and verify comments are preserved.
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	content := string(data)

	// Check that comments are preserved.
	assert.Contains(t, content, "# This is a top-level comment", "top-level comment should be preserved")
	assert.Contains(t, content, "# UI configuration section", "UI section comment should be preserved")
	assert.Contains(t, content, "# Theme setting", "theme comment should be preserved")

	// Check that the trust entry was added.
	assert.Contains(t, content, "/path/to/project", "trusted project path should be present")
}
