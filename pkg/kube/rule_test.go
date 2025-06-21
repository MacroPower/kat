package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

func TestNewRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ruleName string
		match    string
		profile  string
		wantErr  bool
	}{
		{
			name:     "valid rule",
			ruleName: "kustomize",
			match:    "/kustomization\\.ya?ml$",
			profile:  "ks",
			wantErr:  false,
		},
		{
			name:     "valid rule with simple regex",
			ruleName: "yaml",
			match:    "\\.ya?ml$",
			profile:  "yaml",
			wantErr:  false,
		},
		{
			name:     "invalid regex",
			ruleName: "invalid",
			match:    "[",
			profile:  "test",
			wantErr:  true,
		},
		{
			name:     "empty match",
			ruleName: "empty",
			match:    "",
			profile:  "test",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule, err := kube.NewRule(tt.ruleName, tt.match, tt.profile)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, rule)
				assert.Contains(t, err.Error(), tt.ruleName)
			} else {
				require.NoError(t, err)
				require.NotNil(t, rule)
				assert.Equal(t, tt.match, rule.Match)
				assert.Equal(t, tt.profile, rule.Profile)
			}
		})
	}
}

func TestMustNewRule(t *testing.T) {
	t.Parallel()

	t.Run("valid rule", func(t *testing.T) {
		t.Parallel()

		rule := kube.MustNewRule("test", "\\.ya?ml$", "yaml")
		require.NotNil(t, rule)
		assert.Equal(t, "\\.ya?ml$", rule.Match)
		assert.Equal(t, "yaml", rule.Profile)
	})

	t.Run("invalid rule panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			kube.MustNewRule("test", "[", "yaml")
		})
	})
}

func TestRule_CompileMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		match   string
		wantErr bool
	}{
		{
			name:    "valid regex",
			match:   "\\.ya?ml$",
			wantErr: false,
		},
		{
			name:    "complex regex",
			match:   "/kustomization\\.ya?ml$",
			wantErr: false,
		},
		{
			name:    "invalid regex",
			match:   "[",
			wantErr: true,
		},
		{
			name:    "empty regex",
			match:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := &kube.Rule{
				Match:   tt.match,
				Profile: "test",
			}

			err := rule.CompileMatch()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "compile match regex")
			} else {
				require.NoError(t, err)
				// Calling CompileMatch again should not cause an error.
				err2 := rule.CompileMatch()
				require.NoError(t, err2)
			}
		})
	}
}

func TestRule_MatchPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		match    string
		path     string
		expected bool
	}{
		{
			name:     "yaml file matches",
			match:    "\\.ya?ml$",
			path:     "test.yaml",
			expected: true,
		},
		{
			name:     "yml file matches",
			match:    "\\.ya?ml$",
			path:     "test.yml",
			expected: true,
		},
		{
			name:     "kustomization yaml matches",
			match:    "/kustomization\\.ya?ml$",
			path:     "path/to/kustomization.yaml",
			expected: true,
		},
		{
			name:     "kustomization yml matches",
			match:    "/kustomization\\.ya?ml$",
			path:     "path/to/kustomization.yml",
			expected: true,
		},
		{
			name:     "chart yaml matches",
			match:    "/Chart\\.ya?ml$",
			path:     "helm/Chart.yaml",
			expected: true,
		},
		{
			name:     "txt file does not match yaml regex",
			match:    "\\.ya?ml$",
			path:     "test.txt",
			expected: false,
		},
		{
			name:     "partial kustomization path does not match",
			match:    "/kustomization\\.ya?ml$",
			path:     "kustomization-base.yaml",
			expected: false,
		},
		{
			name:     "case sensitive chart match fails",
			match:    "/Chart\\.ya?ml$",
			path:     "helm/chart.yaml",
			expected: false,
		},
		{
			name:     "empty path",
			match:    "\\.ya?ml$",
			path:     "",
			expected: false,
		},
		{
			name:     "root file matches",
			match:    "^[^/]*\\.ya?ml$",
			path:     "config.yaml",
			expected: true,
		},
		{
			name:     "nested file does not match root pattern",
			match:    "^[^/]*\\.ya?ml$",
			path:     "dir/config.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule, err := kube.NewRule("test", tt.match, "profile")
			require.NoError(t, err)

			result := rule.MatchPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRule_MatchPath_Panics(t *testing.T) {
	t.Parallel()

	rule := &kube.Rule{
		Match:   "\\.ya?ml$",
		Profile: "test",
	}
	// Don't compile the match expression.

	assert.Panics(t, func() {
		rule.MatchPath("test.yaml")
	})
}

func TestRule_GetProfile(t *testing.T) {
	t.Parallel()

	t.Run("returns profile when set", func(t *testing.T) {
		t.Parallel()

		profile := &kube.Profile{
			Command: "kubectl",
			Args:    []string{"apply", "-f", "-"},
		}

		rule := &kube.Rule{
			Match:   "\\.ya?ml$",
			Profile: "kubectl",
		}
		rule.SetProfile(profile)

		result := rule.GetProfile()
		assert.Same(t, profile, result)
	})

	t.Run("panics when profile not set", func(t *testing.T) {
		t.Parallel()

		rule := &kube.Rule{
			Match:   "\\.ya?ml$",
			Profile: "kubectl",
		}

		assert.Panics(t, func() {
			rule.GetProfile()
		})
	})
}

func TestRule_SetProfile(t *testing.T) {
	t.Parallel()

	profile := &kube.Profile{
		Command: "helm",
		Args:    []string{"template", "."},
	}

	rule := &kube.Rule{
		Match:   "/Chart\\.ya?ml$",
		Profile: "helm",
	}

	rule.SetProfile(profile)

	// Verify profile was set correctly.
	result := rule.GetProfile()
	assert.Same(t, profile, result)
}

func TestRule_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		command  string
		profile  string
		expected string
		args     []string
	}{
		{
			name:     "helm command",
			command:  "helm",
			args:     []string{"template", ".", "--generate-name"},
			profile:  "helm",
			expected: "helm: helm template . --generate-name",
		},
		{
			name:     "kubectl command",
			command:  "kubectl",
			args:     []string{"apply", "-f", "-"},
			profile:  "kubectl",
			expected: "kubectl: kubectl apply -f -",
		},
		{
			name:     "command with no args",
			command:  "cat",
			args:     []string{},
			profile:  "cat",
			expected: "cat: cat ",
		},
		{
			name:     "kustomize command",
			command:  "kustomize",
			args:     []string{"build", "."},
			profile:  "ks",
			expected: "ks: kustomize build .",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile := &kube.Profile{
				Command: tt.command,
				Args:    tt.args,
			}

			rule := &kube.Rule{
				Match:   "\\.ya?ml$",
				Profile: tt.profile,
			}
			rule.SetProfile(profile)

			result := rule.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRule_String_Panics(t *testing.T) {
	t.Parallel()

	rule := &kube.Rule{
		Match:   "\\.ya?ml$",
		Profile: "test",
	}
	// Don't set the profile.

	assert.Panics(t, func() {
		_ = rule.String()
	})
}
