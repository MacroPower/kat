package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/config"
)

func TestPolicy_IsTrusted(t *testing.T) {
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

			policy := config.NewPolicy()

			if tc.trustedPaths == nil {
				policy.Projects = nil
			} else {
				for _, path := range tc.trustedPaths {
					policy.Projects.Trust = append(policy.Projects.Trust, &config.TrustedProject{Path: path})
				}
			}

			got := policy.IsTrusted(tc.checkPath)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPolicy_TrustProject(t *testing.T) {
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

			// Create a temp policy file.
			dir := t.TempDir()
			policyPath := filepath.Join(dir, "policy.yaml")

			policy := config.NewPolicy()

			// Set up initial paths.
			for _, path := range tc.initialPaths {
				policy.Projects.Trust = append(policy.Projects.Trust, &config.TrustedProject{Path: path})
			}

			// Write initial policy.
			b, err := policy.MarshalYAML()
			require.NoError(t, err)

			err = os.WriteFile(policyPath, b, 0o600)
			require.NoError(t, err)

			// Trust the new project.
			err = policy.TrustProject(tc.addPath, policyPath)
			require.NoError(t, err)

			// Verify in-memory state.
			assert.Len(t, policy.Projects.Trust, tc.wantCount)

			found := false
			for _, tp := range policy.Projects.Trust {
				if filepath.Clean(tp.Path) == filepath.Clean(tc.wantContains) {
					found = true

					break
				}
			}

			assert.True(t, found, "expected path %q to be in trust list", tc.wantContains)

			// Verify persisted state by reloading.
			pl, err := config.NewPolicyLoaderFromFile(policyPath)
			require.NoError(t, err)

			reloaded, err := pl.Load()
			require.NoError(t, err)
			assert.Len(t, reloaded.Projects.Trust, tc.wantCount)
		})
	}
}

func TestPolicy_TrustProject_NilProjects(t *testing.T) {
	t.Parallel()

	// Create a temp policy file.
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")

	policy := config.NewPolicy()

	// Write initial policy.
	b, err := policy.MarshalYAML()
	require.NoError(t, err)

	err = os.WriteFile(policyPath, b, 0o600)
	require.NoError(t, err)

	// Set projects to nil after writing.
	policy.Projects = nil

	// Trust a project.
	err = policy.TrustProject("/path/to/project", policyPath)
	require.NoError(t, err)

	assert.NotNil(t, policy.Projects)
	assert.Len(t, policy.Projects.Trust, 1)
}

func TestPolicy_TrustProject_PreservesComments(t *testing.T) {
	t.Parallel()

	// Create a temp policy file with comments.
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")

	initialPolicy := `# This is a top-level comment
apiVersion: kat.jacobcolvin.com/v1beta1
kind: Policy
# Projects configuration section
projects:
  trust: []
`

	err := os.WriteFile(policyPath, []byte(initialPolicy), 0o600)
	require.NoError(t, err)

	// Load and trust a project.
	pl, err := config.NewPolicyLoaderFromFile(policyPath)
	require.NoError(t, err)

	policy, err := pl.Load()
	require.NoError(t, err)

	err = policy.TrustProject("/path/to/project", policyPath)
	require.NoError(t, err)

	// Read the file back and verify comments are preserved.
	data, err := os.ReadFile(policyPath)
	require.NoError(t, err)

	content := string(data)

	// Check that top-level comments are preserved.
	assert.Contains(t, content, "# This is a top-level comment", "top-level comment should be preserved")
	assert.Contains(t, content, "# Projects configuration section", "projects section comment should be preserved")

	// Check that the trust entry was added.
	assert.Contains(t, content, "/path/to/project", "trusted project path should be present")
}

func TestGetPolicyPath(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv.
	tests := map[string]struct {
		xdgHome string
		home    string
		want    string
	}{
		"uses XDG_CONFIG_HOME when set": {
			xdgHome: "/custom/config",
			home:    "/home/user",
			want:    "/custom/config/kat/policy.yaml",
		},
		"falls back to HOME when XDG not set": {
			xdgHome: "",
			home:    "/home/user",
			want:    "/home/user/.config/kat/policy.yaml",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set test values.
			if tc.xdgHome != "" {
				t.Setenv("XDG_CONFIG_HOME", tc.xdgHome)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			t.Setenv("HOME", tc.home)

			got := config.GetPolicyPath()

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPolicyLoader_Load(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		want    *config.Policy
		input   string
		wantErr bool
	}{
		"loads valid policy": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Policy
projects:
  trust:
    - path: /path/to/project
`,
			want: func() *config.Policy {
				p := config.NewPolicy()
				p.Projects.Trust = []*config.TrustedProject{{Path: "/path/to/project"}}

				return p
			}(),
			wantErr: false,
		},
		"loads empty policy": {
			input: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Policy
`,
			want:    config.NewPolicy(),
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pl := config.NewPolicyLoaderFromBytes([]byte(tc.input))

			got, err := pl.Load()
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want.APIVersion, got.APIVersion)
			assert.Equal(t, tc.want.Kind, got.Kind)
			assert.Len(t, got.Projects.Trust, len(tc.want.Projects.Trust))
		})
	}
}

func TestDefaultPolicyYAMLIsValid(t *testing.T) {
	t.Parallel()

	// Create a temp file with default policy content.
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")

	err := config.WriteDefaultPolicy(policyPath, false)
	require.NoError(t, err)

	// Load and validate.
	pl, err := config.NewPolicyLoaderFromFile(policyPath)
	require.NoError(t, err)

	err = pl.Validate()
	require.NoError(t, err)

	policy, err := pl.Load()
	require.NoError(t, err)

	assert.Equal(t, "kat.jacobcolvin.com/v1beta1", policy.APIVersion)
	assert.Equal(t, "Policy", policy.Kind)
	assert.NotNil(t, policy.Projects)
	assert.Empty(t, policy.Projects.Trust)
}

func TestWriteDefaultPolicy(t *testing.T) {
	t.Parallel()

	t.Run("creates policy file when not exists", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		policyPath := filepath.Join(dir, "policy.yaml")

		err := config.WriteDefaultPolicy(policyPath, false)
		require.NoError(t, err)

		_, err = os.Stat(policyPath)
		assert.NoError(t, err)
	})

	t.Run("does not overwrite existing file without force", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		policyPath := filepath.Join(dir, "policy.yaml")

		// Write custom content.
		customContent := "custom content"
		err := os.WriteFile(policyPath, []byte(customContent), 0o600)
		require.NoError(t, err)

		// Try to write default (should not overwrite).
		err = config.WriteDefaultPolicy(policyPath, false)
		require.NoError(t, err)

		// Verify content unchanged.
		data, err := os.ReadFile(policyPath)
		require.NoError(t, err)
		assert.Equal(t, customContent, string(data))
	})

	t.Run("overwrites existing file with force", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		policyPath := filepath.Join(dir, "policy.yaml")

		// Write custom content.
		customContent := "custom content"
		err := os.WriteFile(policyPath, []byte(customContent), 0o600)
		require.NoError(t, err)

		// Write default with force.
		err = config.WriteDefaultPolicy(policyPath, true)
		require.NoError(t, err)

		// Verify content changed.
		data, err := os.ReadFile(policyPath)
		require.NoError(t, err)
		assert.NotEqual(t, customContent, string(data))
		assert.Contains(t, string(data), "apiVersion")
	})
}
