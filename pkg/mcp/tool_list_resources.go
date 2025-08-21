package mcp

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/kube"
)

// ListResourcesParams defines parameters for the list_resources tool.
type ListResourcesParams struct {
	Path string `json:"path" jsonschema:"description=the directory path to operate on, relative to the project root"`
}

// ListResourcesResult contains the result of listing resources.
type ListResourcesResult struct {
	Error         string                  `json:"error,omitempty"`
	StdoutPreview string                  `json:"stdoutPreview,omitempty"`
	StderrPreview string                  `json:"stderrPreview,omitempty"`
	Message       string                  `json:"message"`
	Resources     []kube.ResourceMetadata `json:"resources"`
	ResourceCount int                     `json:"resourceCount"`
}

// createListResourcesResult creates the MCP tool result from ListResourcesResult.
func createListResourcesResult(result ListResourcesResult) *mcp.CallToolResultFor[ListResourcesResult] {
	msg := fmt.Sprintf("Found %d Kubernetes resources.", result.ResourceCount)
	result.Message = msg

	return &mcp.CallToolResultFor[ListResourcesResult]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: msg,
			},
		},
		StructuredContent: result,
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
