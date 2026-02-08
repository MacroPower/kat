package rule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/rule"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		match   string
		profile string
		wantErr bool
	}{
		"valid rule": {
			match:   `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`,
			profile: "ks",
		},
		"valid rule with simple expression": {
			match:   `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			profile: "yaml",
		},
		"invalid CEL expression": {
			match:   "path.invalidFunction()",
			profile: "test",
			wantErr: true,
		},
		"empty match": {
			match:   "",
			profile: "test",
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, err := rule.New(tc.profile, tc.match)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, r)
				assert.Contains(t, err.Error(), tc.match)
			} else {
				require.NoError(t, err)
				require.NotNil(t, r)
				assert.Equal(t, tc.match, r.Match)
				assert.Equal(t, tc.profile, r.Profile)
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

	tcs := map[string]struct {
		match   string
		wantErr bool
	}{
		"valid CEL expression": {
			match: `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
		},
		"complex CEL expression": {
			match: `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`,
		},
		"invalid CEL expression": {
			match:   "path.invalidFunction()",
			wantErr: true,
		},
		"empty expression": {
			match:   "",
			wantErr: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &rule.Rule{
				Match:   tc.match,
				Profile: "test",
			}

			err := r.CompileMatch()

			if tc.wantErr {
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

	tcs := map[string]struct {
		expression string
		files      []string
		want       bool
	}{
		"boolean expression - true": {
			expression: `files.exists(f, pathExt(f) in [".yaml", ".yml"])`,
			files:      []string{"/app/config.yaml", "/app/service.json"},
			want:       true,
		},
		"boolean expression - false": {
			expression: `files.exists(f, pathExt(f) == ".xml")`,
			files:      []string{"/app/config.yaml", "/app/service.json"},
		},
		"simple boolean - true": {
			expression: `true`,
			files:      []string{"/app/config.yaml"},
			want:       true,
		},
		"simple boolean - false": {
			expression: `false`,
			files:      []string{"/app/config.yaml"},
		},
		"non-boolean expression returns false": {
			expression: `files.filter(f, pathExt(f) in [".yaml", ".yml"])`,
			files:      []string{"/app/config.yaml", "/app/service.json"},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, err := rule.New("test-profile", tc.expression)
			require.NoError(t, err)

			gotMatches := r.MatchFiles("/app", tc.files)
			assert.Equal(t, tc.want, gotMatches)
		})
	}
}
