package mcp

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/kube"
)

func newToolListResources() *mcp.Tool {
	return &mcp.Tool{
		Name:  "list_resources",
		Title: "List Resources",
		Description: `Lists all Kubernetes resources that would be rendered by a manifest generator (Helm, Kustomize, etc.) at the specified path.

IMPORTANT: Use this tool first before attempting to inspect any specific Kubernetes resources.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The directory path to operate on, relative to the project root.",
				},
			},
			Required: []string{"path"},
		},
		OutputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"error": {
					Type:        "string",
					Description: "Error message if the operation failed.",
				},
				"stdoutPreview": {
					Type:        "string",
					Description: "Preview of command stdout output.",
				},
				"stderrPreview": {
					Type:        "string",
					Description: "Preview of command stderr output.",
				},
				"message": {
					Type:        "string",
					Description: "Human-readable message about the operation result.",
				},
				"resources": {
					Type:        "array",
					Description: "List of Kubernetes resource metadata.",
					Items:       newResourceMetadataSchema(),
				},
				"resourceCount": {
					Type:        "integer",
					Description: "Total number of resources found.",
				},
			},
			Required: []string{"message", "resources", "resourceCount"},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
	}
}

// ListResourcesParams defines parameters for the list_resources tool.
type ListResourcesParams struct {
	Path string `json:"path"`
}

// ListResourcesResult contains the result of listing resources.
type ListResourcesResult struct {
	Error         string                  `json:"error,omitempty"`
	StdoutPreview string                  `json:"stdoutPreview"`
	StderrPreview string                  `json:"stderrPreview"`
	Message       string                  `json:"message"`
	Resources     []kube.ResourceMetadata `json:"resources"`
	ResourceCount int                     `json:"resourceCount"`
}

// createListResourcesResult creates the MCP tool result from ListResourcesResult.
func createListResourcesResult(result ListResourcesResult) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: result.Message,
			},
		},
		IsError: result.Error != "",
	}
}

// populateResultFromOutput populates the result with data from command output.
func populateResultFromOutput(result *ListResourcesResult, output command.Output) {
	// Add stdout/stderr previews (truncated for readability).
	result.StdoutPreview = truncateString(output.Stdout, 200)
	result.StderrPreview = truncateString(output.Stderr, 200)

	// Process resources.
	if output.Resources != nil {
		result.ResourceCount = len(output.Resources)
		for _, resource := range output.Resources {
			if resource.Object != nil {
				result.Resources = append(result.Resources, resource.Object.GetMetadata())
			}
		}
	}
}
