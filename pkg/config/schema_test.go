package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/config"
)

func TestValidateWithSchema(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlContent string
		errContains string
		wantErr     bool
	}{
		"valid minimal config": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
`,
			wantErr: false,
		},
		"valid full config": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    args: ["hello"]
    source: 'files.filter(f, pathExt(f) == ".yaml")'
    env:
      - name: TEST_VAR
        value: "test"
    envFrom:
      - callerRef:
          pattern: "^TEST_.+"
    hooks:
      init:
        - command: helm
          args: ["version"]
    plugins:
      test-plugin:
        command: echo
        args: ["plugin"]
        description: "Test plugin"
        keys:
          - code: "ctrl+t"
            alias: "⌃t"
rules:
  - match: 'files.exists(f, pathBase(f) == "test.yaml")'
    profile: test
`,
			wantErr: false,
		},
		"missing apiVersion": {
			yamlContent: `kind: Configuration
profiles:
  test:
    command: echo
`,
			wantErr:     true,
			errContains: "missing property 'apiVersion'",
		},
		"missing kind": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
profiles:
  test:
    command: echo
`,
			wantErr:     true,
			errContains: "missing property 'kind'",
		},
		"invalid apiVersion": {
			yamlContent: `apiVersion: invalid/v1
kind: Configuration
`,
			wantErr:     true,
			errContains: "value must be 'kat.jacobcolvin.com/v1beta1'",
		},
		"invalid kind": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: InvalidKind
`,
			wantErr:     true,
			errContains: "value must be 'Configuration'",
		},
		"profile without command": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    args: ["hello"]
`,
			wantErr:     true,
			errContains: "missing property 'command'",
		},
		"rule without match": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
rules:
  - profile: test
`,
			wantErr:     true,
			errContains: "missing property 'match'",
		},
		"rule without profile": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
rules:
  - match: 'files.exists(f, pathBase(f) == "test.yaml")'
`,
			wantErr:     true,
			errContains: "missing property 'profile'",
		},
		"invalid env var - missing name": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    env:
      - value: "test"
`,
			wantErr:     true,
			errContains: "missing property 'name'",
		},
		"hook without command": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    hooks:
      init:
        - args: ["version"]
`,
			wantErr:     true,
			errContains: "missing property 'command'",
		},
		"plugin without command": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    plugins:
      test-plugin:
        description: "Test plugin"
`,
			wantErr:     true,
			errContains: "missing property 'command'",
		},
		"key binding without code": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    plugins:
      test-plugin:
        command: echo
        keys:
          - alias: "⌃t"
`,
			wantErr:     true,
			errContains: "missing property 'code'",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateWithSchema([]byte(tc.yamlContent))

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)

				// Verify that schema validation errors have proper path information
				// when possible (not for YAML parsing errors).
				if tc.errContains != "parse YAML" {
					schemaErr := &config.SchemaValidationError{}
					require.ErrorAs(t, err, &schemaErr)
					if tc.errContains != "parse YAML" {
						assert.NotNil(t, schemaErr.Path, "Expected path information for validation error")
					}
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidationErrorAnnotateSource(t *testing.T) {
	t.Parallel()

	yamlContent := `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  invalid:
    # missing required command field
    args: ["test"]
`

	err := config.ValidateWithSchema([]byte(yamlContent))
	require.Error(t, err)

	schemaErr := &config.SchemaValidationError{}
	require.ErrorAs(t, err, &schemaErr)
	require.NotNil(t, schemaErr.Path)

	// Test that AnnotateSource works with the path.
	source, annotateErr := schemaErr.Path.AnnotateSource([]byte(yamlContent), true)
	require.NoError(t, annotateErr)

	// Convert to string to check contents (since AnnotateSource returns colorized output).
	sourceStr := string(source)
	assert.Contains(t, sourceStr, "profiles")
	assert.Contains(t, sourceStr, "invalid")
}
