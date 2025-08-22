package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/version"
)

// ExecutionState tracks the current state of command execution.
type ExecutionState struct {
	Output command.Output
}

type CommandRunner interface {
	Subscribe(ch chan<- command.Event)
	ConfigureContext(ctx context.Context, opts ...command.RunnerOpt) error
	RunContext(ctx context.Context) command.Output
	SendEvent(evt command.Event)
}

// Server implements the MCP server for kat.
type Server struct {
	runner         CommandRunner
	tracer         trace.Tracer
	completionCond *sync.Cond
	server         *mcp.Server
	eventCh        chan command.Event
	address        string
	currentPath    string
	state          ExecutionState
	mu             sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(address string, runner CommandRunner, initialPath string) (*Server, error) {
	impl := &mcp.Implementation{
		Name:    "kat",
		Title:   "kat",
		Version: version.GetVersion(),
	}

	opts := &mcp.ServerOptions{
		Instructions: `MCP Server 'kat' enables rendering, validating, and browsing Kubernetes manifests from Helm charts, Kustomize overlays, and any other manifest generators.

Unless otherwise specified in instructions, 'kat' should be used with ANY manifest generator that produces Kubernetes YAML, including but not limited to:
- Helm charts, Kustomize overlays, raw YAML files
- KCL, CUE, Jsonnet, Dhall configurations
- Experimental or proprietary manifest generators

Required workflow:
1. ALWAYS use 'list_resources' first with a directory path containing Kubernetes manifest sources (e.g., ".", "./helm-chart", "./kustomize-overlay")
2. STOP and carefully READ the output to see all available resources with their metadata
3. Use 'get_resource' to retrieve full YAML content using the EXACT apiVersion, kind, namespace, and name values from 'list_resources' output

When to use these tools:
- Analyzing what resources ANY manifest generator or configuration will produce
- Inspecting the final rendered YAML (regardless of the source format or tool)
- Validating Kubernetes resources before deployment
- Debugging manifest generation issues from any toolchain

The tools automatically detect the project type and invoke the appropriate rendering and validation commands internally.`,
	}

	mcpServer := mcp.NewServer(impl, opts)

	s := &Server{
		address:     address,
		server:      mcpServer,
		runner:      runner,
		eventCh:     make(chan command.Event, 100),
		currentPath: initialPath,
		state:       ExecutionState{},
		tracer:      otel.Tracer("mcp-server"),
	}

	s.completionCond = sync.NewCond(&s.mu)

	runner.Subscribe(s.eventCh)

	s.registerTools()

	// Start event processing.
	go s.processEvents()

	return s, nil
}

// registerTools registers all available tools with the MCP server.
func (s *Server) registerTools() {
	// Register the list_resources tool.
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list_resources",
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
	}, WithTracing(s.tracer, s.handleListResources))

	// Register the get_resource tool.
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "get_resource",
		Description: `Gets the fully rendered YAML content of a specific Kubernetes resource.

Use this tool to retrieve the YAML representation of a resource after it has been rendered by the manifest generator.

IMPORTANT: You MUST first use 'list_resources' to get available resources, then use the EXACT values from its output.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
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
	}, WithTracing(s.tracer, s.handleGetResource))
}

// processEvents processes command events in a separate goroutine.
func (s *Server) processEvents() {
	for event := range s.eventCh {
		switch e := event.(type) {
		case command.EventEnd:
			s.updateState(e.Output)

			// Broadcast to all waiters.
			s.completionCond.Broadcast()
		}
	}
}

func (s *Server) updateState(output command.Output) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Output = output
}

func (s *Server) pathChanged(newPath string) bool {
	// If no path provided, use current path (no-op).
	if newPath == "" {
		return false
	}

	// If path hasn't changed, this is a no-op.
	if s.currentPath == newPath {
		return false
	}

	return true
}

func (s *Server) reload(ctx context.Context, path string) error {
	ctx, span := s.tracer.Start(ctx, "reload")
	defer span.End()

	if !s.pathChanged(path) {
		return nil // No path change, nothing to do.
	}

	logger := log.WithContext(ctx)
	logger.DebugContext(ctx, "reloading with new path", slog.String("path", path))

	// Reconfigure the runner with the new path.
	err := s.runner.ConfigureContext(ctx,
		command.WithAutoProfile(),
		command.WithPath(path),
	)
	if err != nil {
		return fmt.Errorf("reconfigure runner with path %q: %w", path, err)
	}

	s.currentPath = path

	t := time.Now()

	// Start the command with the new path.
	go s.runner.RunContext(ctx)

	// Wait for any completion that occurs after our request was made.
	err = s.waitForCompletion(ctx, t)
	if err != nil {
		return fmt.Errorf("wait for completion: %w", err)
	}

	logger.DebugContext(ctx, "reload completed successfully")

	return nil
}

// waitForCompletion blocks until any command execution completes or the context is canceled.
func (s *Server) waitForCompletion(ctx context.Context, reloadTime time.Time) error {
	ctx, span := s.tracer.Start(ctx, "wait")
	defer span.End()

	if reloadTime.IsZero() {
		return nil // No reload happened.
	}

	for {
		// Check if the last completion should be allowed.
		if s.state.Output.Timestamp.After(reloadTime) {
			return nil
		}

		// Check if context was canceled.
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for completion canceled: %w", ctx.Err())
		default:
		}

		// Wait for the next completion.
		s.completionCond.Wait()
		// The condition variable's Wait() method atomically releases the mutex and waits.
		// This allows updateState() in another goroutine to acquire the lock.
		// After updateState() broadcasts and releases the lock, Wait() re-acquires it before returning.
	}
}

// handleListResources handles the list_resources tool call.
func (s *Server) handleListResources(
	ctx context.Context,
	_ *mcp.ServerSession,
	params *mcp.CallToolParamsFor[ListResourcesParams],
) (*mcp.CallToolResultFor[ListResourcesResult], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.reload(ctx, params.Arguments.Path)
	if err != nil {
		return nil, fmt.Errorf("reconfigure runner: %w", err)
	}

	result := ListResourcesResult{
		Resources: []kube.ResourceMetadata{},
	}

	if s.state.Output.Error != nil {
		result.Error = s.state.Output.Error.Error()
	}

	populateResultFromOutput(&result, s.state.Output)

	s.runner.SendEvent(command.NewEventListResources(ctx))

	return createListResourcesResult(result), nil
}

// handleGetResource handles the get_resource tool call.
func (s *Server) handleGetResource(
	ctx context.Context,
	_ *mcp.ServerSession,
	params *mcp.CallToolParamsFor[GetResourceParams],
) (*mcp.CallToolResultFor[GetResourceResult], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.reload(ctx, params.Arguments.Path)
	if err != nil {
		return nil, fmt.Errorf("reconfigure runner: %w", err)
	}

	result := GetResourceResult{
		Found: false,
	}

	if s.state.Output.Error != nil {
		result.Error = s.state.Output.Error.Error()
	}

	// Search for the requested resource.
	resource := findResource(s.state.Output.Resources, params.Arguments)
	if resource != nil {
		result.Found = true
		result.Resource = &ResourceDetails{
			Metadata: resource.Object.GetMetadata(),
			YAML:     resource.YAML,
		}

		// Send event to open the resource in the pager.
		s.runner.SendEvent(command.NewEventOpenResource(ctx, *resource))
	}

	return createGetResourceResult(result, params.Arguments), nil
}

func (s *Server) Server() *mcp.Server {
	return s.server
}

func (s *Server) Close() {
	close(s.eventCh)
}

// Serve starts the MCP server.
func (s *Server) Serve(ctx context.Context) error {
	slog.InfoContext(ctx, "starting MCP server", slog.String("address", s.address))

	if s.address == "" {
		err := s.serveStdio(ctx)
		if err != nil {
			return fmt.Errorf("serve Stdio: %w", err)
		}

		return nil
	}

	err := s.serveHTTP()
	if err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

func (s *Server) serveHTTP() error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.server
	}, nil)

	server := &http.Server{
		Addr:    s.address,
		Handler: handler,

		ReadHeaderTimeout: 10 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

func (s *Server) serveStdio(ctx context.Context) error {
	t := mcp.NewLoggingTransport(mcp.NewStdioTransport(), os.Stderr)
	err := s.server.Run(ctx, t)
	if err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
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
