// Package v1beta1 contains the v1beta1 API types for kat configuration.
package v1beta1

import "github.com/invopop/jsonschema"

// APIVersion is the current API version for all kat configuration kinds.
const APIVersion = "kat.jacobcolvin.com/v1beta1"

// ValidAPIVersions contains all valid API versions.
var ValidAPIVersions = []string{APIVersion}

// TypeMeta contains the API version and kind metadata common to all config types.
type TypeMeta struct {
	// APIVersion specifies the API version for this configuration.
	APIVersion string `json:"apiVersion" jsonschema:"title=API Version"`
	// Kind defines the type of configuration.
	Kind string `json:"kind" jsonschema:"title=Kind"`
}

// GetAPIVersion returns the API version.
func (tm TypeMeta) GetAPIVersion() string {
	return tm.APIVersion
}

// GetKind returns the kind.
func (tm TypeMeta) GetKind() string {
	return tm.Kind
}

// Object is the interface that all config types implement.
type Object interface {
	GetAPIVersion() string
	GetKind() string
	EnsureDefaults()
}

// ExtendSchemaWithEnums adds apiVersion and kind enum constraints to a JSON schema.
func ExtendSchemaWithEnums(jss *jsonschema.Schema, apiVersions, kinds []string) {
	apiVersion, ok := jss.Properties.Get("apiVersion")
	if !ok {
		panic("apiVersion property not found in schema")
	}

	for _, version := range apiVersions {
		apiVersion.OneOf = append(apiVersion.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: version,
			Title: "API Version",
		})
	}

	_, _ = jss.Properties.Set("apiVersion", apiVersion)

	kind, ok := jss.Properties.Get("kind")
	if !ok {
		panic("kind property not found in schema")
	}

	for _, kindValue := range kinds {
		kind.OneOf = append(kind.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: kindValue,
			Title: "Kind",
		})
	}

	_, _ = jss.Properties.Set("kind", kind)
}
