package yaml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/yaml"
)

func TestMergeRootFromValue(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		value   any
		input   string
		want    string
		errMsg  string
		wantErr bool
	}{
		"merge adds new fields": {
			input: `existing: value
`,
			value: map[string]string{"new": "field"},
			want: `existing: value
new: field
`,
			wantErr: false,
		},
		"merge overwrites existing fields": {
			input: `key: old
`,
			value: map[string]string{"key": "new"},
			want: `key: new
`,
			wantErr: false,
		},
		"merge preserves comments": {
			input: `# Top comment
key: value
`,
			value:   map[string]string{"new": "field"},
			want:    "# Top comment\nkey: value\nnew: field\n",
			wantErr: false,
		},
		"merge with nested map": {
			input: `top: existing
`,
			value: map[string]any{
				"nested": map[string]string{
					"field": "value",
				},
			},
			want: `top: existing
nested:
  field: value
`,
			wantErr: false,
		},
		"empty document returns error": {
			input:   ``,
			value:   map[string]string{"key": "value"},
			wantErr: true,
			errMsg:  "merge yaml",
		},
		"invalid YAML input": {
			input:   `invalid: [yaml`,
			value:   map[string]string{"key": "value"},
			wantErr: true,
			errMsg:  "parse yaml",
		},
		"nil value returns error": {
			input:   `key: value`,
			value:   nil,
			wantErr: true,
			errMsg:  "merge yaml",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := yaml.MergeRootFromValue([]byte(tc.input), tc.value)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}
