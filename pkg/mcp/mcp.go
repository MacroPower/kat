package mcp

import "github.com/modelcontextprotocol/go-sdk/jsonschema"

const (
	name         = "kat"
	instructions = `MCP Server 'kat' enables rendering, validating, and browsing Kubernetes manifests from Helm charts, Kustomize overlays, and any other manifest generators.

When to use these tools:
- Analyzing what resources ANY manifest generator or configuration will produce
- Inspecting and validating the final rendered YAML (regardless of the source format or tool)
- Debugging manifest generation issues from any toolchain
- Observing changes after modifying values, configs, templates, or any other manifest sources

REQUIRED workflow:
1. Use 'list_resources' first with a directory path containing Kubernetes manifest sources (e.g., ".", "./helm-chart", "./kustomize-overlay")
2. STOP and carefully READ the output to see all available resources with their metadata
3. Use 'get_resource' to retrieve full YAML content using the EXACT apiVersion, kind, namespace, and name values from 'list_resources' output
4. EXCEPTION: If you modify a single resource without changing its metadata, you are allowed to repeatedly call 'get_resource' to retrieve the updated YAML

IMPORTANT: When making edits to a single resource, you MUST call 'get_resource' on the affected resource BOTH BEFORE AND AFTER you make changes.
`
)

func newResourceMetadataSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "object",
		Description: "Metadata of a Kubernetes resource.",
		Properties: map[string]*jsonschema.Schema{
			"apiVersion": {
				Type:        "string",
				Description: "The API version of the resource.",
			},
			"kind": {
				Type:        "string",
				Description: "The kind of the resource.",
			},
			"namespace": {
				Type:        "string",
				Description: "The namespace of the resource. Empty string for resources without a namespace.",
			},
			"name": {
				Type:        "string",
				Description: "The name of the resource.",
			},
		},
		Required: []string{"apiVersion", "kind", "namespace", "name"},
	}
}

// truncateString truncates a string to maxLen characters with ellipsis if needed.
func truncateString(str string, maxLen int) string {
	if str == "" {
		return ""
	}
	if len(str) > maxLen {
		return str[:maxLen] + "\n[OUTPUT TRUNCATED]"
	}

	return str
}
