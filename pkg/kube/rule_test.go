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
		name    string
		match   string
		profile string
		wantErr bool
	}{
		{
			name:    "valid rule",
			match:   `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`,
			profile: "ks",
			wantErr: false,
		},
		{
			name:    "valid rule with simple expression",
			match:   `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			profile: "yaml",
			wantErr: false,
		},
		{
			name:    "invalid CEL expression",
			match:   "path.invalidFunction()",
			profile: "test",
			wantErr: true,
		},
		{
			name:    "empty match",
			match:   "",
			profile: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule, err := kube.NewRule(tt.profile, tt.match)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, rule)
				assert.Contains(t, err.Error(), tt.match)
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

		rule := kube.MustNewRule("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`)
		require.NotNil(t, rule)
		assert.Equal(t, `files.exists(f, pathExt(f) in [".yaml", ".yml"])`, rule.Match)
		assert.Equal(t, "yaml", rule.Profile)
	})

	t.Run("invalid rule panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			kube.MustNewRule("yaml", "path.invalidFunction()")
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
			name:    "valid CEL expression",
			match:   `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			wantErr: false,
		},
		{
			name:    "complex CEL expression",
			match:   `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`,
			wantErr: false,
		},
		{
			name:    "invalid CEL expression",
			match:   "path.invalidFunction()",
			wantErr: true,
		},
		{
			name:    "empty expression",
			match:   "",
			wantErr: true,
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
				assert.Contains(t, err.Error(), "compile match expression")
			} else {
				require.NoError(t, err)
				// Calling CompileMatch again should not cause an error.
				err2 := rule.CompileMatch()
				require.NoError(t, err2)
			}
		})
	}
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

func TestRule_MatchFiles_BooleanAndLegacySupport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		expression  string
		files       []string
		wantMatches bool
	}{
		{
			name:        "boolean expression - true",
			expression:  `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			files:       []string{"/app/config.yaml", "/app/service.json"},
			wantMatches: true,
		},
		{
			name:        "boolean expression - false",
			expression:  `files.exists(f, pathExt(f) == ".xml")`,
			files:       []string{"/app/config.yaml", "/app/service.json"},
			wantMatches: false,
		},
		{
			name:        "simple boolean - true",
			expression:  `true`,
			files:       []string{"/app/config.yaml"},
			wantMatches: true,
		},
		{
			name:        "simple boolean - false",
			expression:  `false`,
			files:       []string{"/app/config.yaml"},
			wantMatches: false,
		},
		{
			name:        "non-boolean expression returns false",
			expression:  `files.filter(f, pathExt(f) in [".yaml", ".yml"])`,
			files:       []string{"/app/config.yaml", "/app/service.json"},
			wantMatches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule, err := kube.NewRule("test-profile", tt.expression)
			require.NoError(t, err)

			gotMatches := rule.MatchFiles("/app", tt.files)
			assert.Equal(t, tt.wantMatches, gotMatches)
		})
	}
}
