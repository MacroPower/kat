package rule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/rule"
)

func TestNew(t *testing.T) {
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

			r, err := rule.New(tt.profile, tt.match)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, r)
				assert.Contains(t, err.Error(), tt.match)
			} else {
				require.NoError(t, err)
				require.NotNil(t, r)
				assert.Equal(t, tt.match, r.Match)
				assert.Equal(t, tt.profile, r.Profile)
			}
		})
	}
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	t.Run("valid rule", func(t *testing.T) {
		t.Parallel()

		r := rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`)
		require.NotNil(t, r)
		assert.Equal(t, `files.exists(f, pathExt(f) in [".yaml", ".yml"])`, r.Match)
		assert.Equal(t, "yaml", r.Profile)
	})

	t.Run("invalid rule panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			rule.MustNew("yaml", "path.invalidFunction()")
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

			r := &rule.Rule{
				Match:   tt.match,
				Profile: "test",
			}

			err := r.CompileMatch()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "compile match expression")
			} else {
				require.NoError(t, err)
				// Calling CompileMatch again should not cause an error.
				err2 := r.CompileMatch()
				require.NoError(t, err2)
			}
		})
	}
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

			r, err := rule.New("test-profile", tt.expression)
			require.NoError(t, err)

			gotMatches := r.MatchFiles("/app", tt.files)
			assert.Equal(t, tt.wantMatches, gotMatches)
		})
	}
}
