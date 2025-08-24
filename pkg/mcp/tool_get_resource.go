package mcp

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/kube"
)

func newToolGetResource() *mcp.Tool {
	return &mcp.Tool{
		Name:  "get_resource",
		Title: "Get Resource",
		Description: `Gets the fully rendered YAML content of a specific Kubernetes resource.

Use this tool to retrieve the YAML representation of a resource after it has been rendered by the manifest generator.

IMPORTANT: Since resources can be very large, only call 'get_resource' on specific resources that you need to observe. Unnecessarily reading many different resources will negatively impact your performance.

IMPORTANT: The most recent 'get_resource' call will be shown to the user. If you're about to edit a resource, ALWAYS call 'get_resource' immediately before making your change, so the user can observe the diff.

IMPORTANT: You MUST first use 'list_resources' to get available resources, then use the EXACT values from its output.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"apiVersion": {
					Type:        "string",
					Description: "The API version of the resource (e.g. v1 or apps/v1).",
				},
				"kind": {
					Type:        "string",
					Description: "The kind of the resource (e.g. Pod or Deployment).",
				},
				"namespace": {
					Type:        "string",
					Description: "The namespace of the resource. Use an empty string for resources without a namespace.",
				},
				"name": {
					Type:        "string",
					Description: "The name of the resource.",
				},
				"path": {
					Type:        "string",
					Description: "The directory path to operate on, relative to the project root.",
				},
			},
			Required: []string{"apiVersion", "kind", "namespace", "name", "path"},
		},
		OutputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"resource": {
					Type:        "object",
					Description: "The Kubernetes resource details.",
					Properties: map[string]*jsonschema.Schema{
						"metadata": newResourceMetadataSchema(),
						"yaml": {
							Type:        "string",
							Description: "The YAML representation of the resource.",
						},
					},
				},
				"error": {
					Type:        "string",
					Description: "Error message if the operation failed.",
				},
				"message": {
					Type:        "string",
					Description: "Human-readable message about the operation result.",
				},
				"found": {
					Type:        "boolean",
					Description: "Whether the resource was found.",
				},
			},
			Required: []string{"message", "found"},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
	}
}

// GetResourceParams defines parameters for the get_resource tool.
type GetResourceParams struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Path       string `json:"path"`
}

// GetResourceResult contains the result of getting a single resource.
type GetResourceResult struct {
	Resource *ResourceDetails `json:"resource,omitempty"`
	Error    string           `json:"error,omitempty"`
	Message  string           `json:"message"`
	Found    bool             `json:"found"`
}

// ResourceDetails contains detailed information about a Kubernetes resource.
type ResourceDetails struct {
	Metadata kube.ResourceMetadata `json:"metadata"`
	YAML     string                `json:"yaml"`
}

// createGetResourceResult creates the MCP tool result from GetResourceResult.
func createGetResourceResult(result GetResourceResult) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result.Message},
		},
		IsError: result.Error != "",
	}
}

// formatResourceMessage formats the message for the get_resource tool result.
func formatResourceMessage(result GetResourceResult, params GetResourceParams) string {
	resourceID := formatResourceID(params)

	if result.Found {
		return fmt.Sprintf("Found resource %s.", resourceID)
	}

	if result.Error != "" {
		return result.Error
	}

	return fmt.Sprintf(
		"INVALID INPUT ERROR: Resource %s not found. Use an EXACT INPUT from the list_resources tool.",
		resourceID,
	)
}

// formatResourceID formats a resource identifier string.
func formatResourceID(params GetResourceParams) string {
	if params.Namespace != "" {
		return fmt.Sprintf("%s/%s %s/%s",
			params.APIVersion, params.Kind, params.Namespace, params.Name)
	}

	return fmt.Sprintf("%s/%s %s", params.APIVersion, params.Kind, params.Name)
}

// findResource searches for a resource matching the given parameters.
func findResource(resources []*kube.Resource, params GetResourceParams) *kube.Resource {
	if resources == nil {
		return nil
	}

	for _, resource := range resources {
		if resource.Object == nil {
			continue
		}

		obj := resource.Object

		// Match API version.
		if obj.GetAPIVersion() != params.APIVersion {
			continue
		}

		// Match kind.
		if obj.GetKind() != params.Kind {
			continue
		}

		// Match name.
		if obj.GetName() != params.Name {
			continue
		}

		// Match namespace.
		if obj.GetNamespace() != params.Namespace {
			continue
		}

		// All criteria match.
		return resource
	}

	return nil
}
