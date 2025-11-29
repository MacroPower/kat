package v1beta1_test

import (
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/api/v1beta1"
)

func TestTypeMeta_GetAPIVersion(t *testing.T) {
	t.Parallel()

	tm := v1beta1.TypeMeta{
		APIVersion: "test.example.com/v1",
		Kind:       "TestKind",
	}

	got := tm.GetAPIVersion()

	assert.Equal(t, "test.example.com/v1", got)
}

func TestTypeMeta_GetKind(t *testing.T) {
	t.Parallel()

	tm := v1beta1.TypeMeta{
		APIVersion: "test.example.com/v1",
		Kind:       "TestKind",
	}

	got := tm.GetKind()

	assert.Equal(t, "TestKind", got)
}

func TestExtendSchemaWithEnums(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		apiVersions     []string
		kinds           []string
		wantAPIVersions int
		wantKinds       int
	}{
		"single API version": {
			apiVersions:     []string{"v1"},
			kinds:           []string{"Kind1"},
			wantAPIVersions: 1,
			wantKinds:       1,
		},
		"multiple API versions": {
			apiVersions:     []string{"v1", "v1beta1", "v1alpha1"},
			kinds:           []string{"Kind1"},
			wantAPIVersions: 3,
			wantKinds:       1,
		},
		"multiple kinds": {
			apiVersions:     []string{"v1"},
			kinds:           []string{"Kind1", "Kind2", "Kind3"},
			wantAPIVersions: 1,
			wantKinds:       3,
		},
		"combined API versions and kinds": {
			apiVersions:     []string{"v1", "v1beta1"},
			kinds:           []string{"Kind1", "Kind2"},
			wantAPIVersions: 2,
			wantKinds:       2,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create a schema with apiVersion and kind properties.
			jss := &jsonschema.Schema{
				Properties: jsonschema.NewProperties(),
			}
			jss.Properties.Set("apiVersion", &jsonschema.Schema{Type: "string"})
			jss.Properties.Set("kind", &jsonschema.Schema{Type: "string"})

			v1beta1.ExtendSchemaWithEnums(jss, tc.apiVersions, tc.kinds)

			apiVersion, ok := jss.Properties.Get("apiVersion")
			assert.True(t, ok)
			assert.Len(t, apiVersion.OneOf, tc.wantAPIVersions)

			kind, ok := jss.Properties.Get("kind")
			assert.True(t, ok)
			assert.Len(t, kind.OneOf, tc.wantKinds)

			// Verify the const values are set correctly.
			for i, v := range tc.apiVersions {
				assert.Equal(t, v, apiVersion.OneOf[i].Const)
			}

			for i, k := range tc.kinds {
				assert.Equal(t, k, kind.OneOf[i].Const)
			}
		})
	}
}

func TestExtendSchemaWithEnums_PanicsWithoutAPIVersion(t *testing.T) {
	t.Parallel()

	jss := &jsonschema.Schema{
		Properties: jsonschema.NewProperties(),
	}
	jss.Properties.Set("kind", &jsonschema.Schema{Type: "string"})

	assert.Panics(t, func() {
		v1beta1.ExtendSchemaWithEnums(jss, []string{"v1"}, []string{"Kind1"})
	})
}

func TestExtendSchemaWithEnums_PanicsWithoutKind(t *testing.T) {
	t.Parallel()

	jss := &jsonschema.Schema{
		Properties: jsonschema.NewProperties(),
	}
	jss.Properties.Set("apiVersion", &jsonschema.Schema{Type: "string"})

	assert.Panics(t, func() {
		v1beta1.ExtendSchemaWithEnums(jss, []string{"v1"}, []string{"Kind1"})
	})
}
