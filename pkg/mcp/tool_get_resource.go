package mcp

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/kube"
)

// GetResourceParams defines parameters for the get_resource tool.
type GetResourceParams struct {
	APIVersion string `json:"apiVersion"          jsonschema:"the API version of the resource (e.g. v1 or apps/v1)"`
	Kind       string `json:"kind"                jsonschema:"the kind of the resource (e.g. Pod or Deployment)"`
	Name       string `json:"name"                jsonschema:"the name of the resource"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"the namespace of the resource (optional for cluster-scoped resources)"`
}

// GetResourceResult contains the result of getting a single resource.
type GetResourceResult struct {
	Resource *ResourceDetails `json:"resource,omitempty"`
	Status   string           `json:"status"`
	Error    string           `json:"error,omitempty"`
	Found    bool             `json:"found"`
}

// ResourceDetails contains detailed information about a Kubernetes resource.
type ResourceDetails struct {
	Metadata kube.ResourceMetadata `json:"metadata"`
	YAML     string                `json:"yaml"`
}

// createGetResourceResult creates the MCP tool result from GetResourceResult.
func createGetResourceResult(
	result GetResourceResult,
	params GetResourceParams,
) *mcp.CallToolResultFor[GetResourceResult] {
	text := formatResourceMessage(result, params)

	return &mcp.CallToolResultFor[GetResourceResult]{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
		StructuredContent: result,
	}
}

// formatResourceMessage formats the message for the get_resource tool result.
func formatResourceMessage(result GetResourceResult, params GetResourceParams) string {
	resourceID := formatResourceID(params)

	if result.Found {
		return fmt.Sprintf("Found resource %s. Status: %s", resourceID, result.Status)
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

		// Match namespace (if specified).
		if params.Namespace != "" && obj.GetNamespace() != params.Namespace {
			continue
		}

		// All criteria match.
		return resource
	}

	return nil
}
