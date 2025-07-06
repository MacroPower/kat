package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWithSchema(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yamlContent string
		wantErr     bool
		errContains string
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
		"invalid env var - both value and valueFrom": {
			yamlContent: `apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
profiles:
  test:
    command: echo
    env:
      - name: TEST_VAR
        value: "test"
        valueFrom:
          callerRef:
            name: OTHER_VAR
`,
			wantErr:     true,
			errContains: "'oneOf' failed, subschemas 0, 1 matched",
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

			err := ValidateWithSchema([]byte(tc.yamlContent))

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)

				// Verify that schema validation errors have proper path information
				// when possible (not for YAML parsing errors).
				if tc.errContains != "parse YAML" {
					schemaErr, ok := err.(*SchemaValidationError)
					assert.True(t, ok, "Expected SchemaValidationError")
					if ok && tc.errContains != "parse YAML" {
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

	err := ValidateWithSchema([]byte(yamlContent))
	require.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	require.NotNil(t, schemaErr.Path)

	// Test that AnnotateSource works with the path.
	source, annotateErr := schemaErr.Path.AnnotateSource([]byte(yamlContent), true)
	require.NoError(t, annotateErr)

	// Convert to string to check contents (since AnnotateSource returns colorized output).
	sourceStr := string(source)
	assert.Contains(t, sourceStr, "profiles")
	assert.Contains(t, sourceStr, "invalid")
}
