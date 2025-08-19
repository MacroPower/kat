package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/version"
)

// ExecutionStatus represents the current state of command execution.
type ExecutionStatus string

const (
	// StatusIdle indicates no command is currently executing.
	StatusIdle ExecutionStatus = "idle"
	// StatusRunning indicates a command is currently executing.
	StatusRunning ExecutionStatus = "running"
	// StatusCompleted indicates the last command completed successfully.
	StatusCompleted ExecutionStatus = "completed"
	// StatusError indicates the last command completed with an error.
	StatusError ExecutionStatus = "error"
)

// ExecutionState tracks the current state of command execution.
type ExecutionState struct {
	Error           error
	Output          *command.Output
	Status          ExecutionStatus
	CompletionCount int64
	RequestCount    int64
}

type CommandRunner interface {
	Subscribe(ch chan<- command.Event)
	Configure(opts ...command.RunnerOpt) error
	RunContext(ctx context.Context) command.Output
}

// Server implements the MCP server for kat.
type Server struct {
	runner          CommandRunner
	server          *mcp.Server
	eventCh         chan command.Event
	completionCond  *sync.Cond
	address         string
	currentPath     string
	state           ExecutionState
	completionCount int64
	requestCount    int64
	mu              sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(address string, runner CommandRunner, initialPath string) (*Server, error) {
	impl := &mcp.Implementation{
		Name:    "kat",
		Version: version.GetVersion(),
	}

	opts := &mcp.ServerOptions{
		Instructions: "MCP Server for rendering and browsing Kubernetes manifests. Workflow: 1) Use the list_resources tool to get a list of resources. 2) STOP and READ the output. 3) Use the get_resource tool to get specific resources from the list_resources output.",
	}

	mcpServer := mcp.NewServer(impl, opts)

	s := &Server{
		address:     address,
		server:      mcpServer,
		runner:      runner,
		eventCh:     make(chan command.Event, 100),
		currentPath: initialPath,
		state: ExecutionState{
			Status: StatusIdle,
		},
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
		Name:        "list_resources",
		Description: "List Kubernetes resources rendered by a project (e.g., helm, kustomize) at a particular path. You MUST specify a path.",
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
	}, s.handleListResources)

	// Register the get_resource tool.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_resource",
		Description: "Get details of a specific Kubernetes resource. You MUST use inputs from a list_resources output in the resources list EXACTLY.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"apiVersion": {
					Type:        "string",
					Description: "The API version of the resource, if applicable",
				},
				"kind": {
					Type:        "string",
					Description: "The kind of the resource",
				},
				"namespace": {
					Type:        "string",
					Description: "The namespace of the resource, if applicable",
				},
				"name": {
					Type:        "string",
					Description: "The name of the resource",
				},
				"path": {
					Type:        "string",
					Description: "The directory path to operate on, relative to the project root.",
				},
			},
			Required: []string{"kind", "name", "path"},
		},
	}, s.handleGetResource)
}

// processEvents processes command events in a separate goroutine.
func (s *Server) processEvents() {
	for event := range s.eventCh {
		s.mu.Lock()

		switch e := event.(type) {
		case command.EventStart:
			s.state.Status = StatusRunning
			s.state.Output = nil
			s.state.Error = nil

		case command.EventEnd:
			completionCount := atomic.AddInt64(&s.completionCount, 1)
			currentRequestCount := atomic.LoadInt64(&s.requestCount)

			if e.Error != nil {
				s.state.Status = StatusError
				s.state.Error = e.Error
			} else {
				s.state.Status = StatusCompleted
				s.state.Error = nil
			}
			// Convert EventEnd to Output.
			output := command.Output(e)
			s.state.Output = &output
			s.state.CompletionCount = completionCount
			s.state.RequestCount = currentRequestCount

			// Broadcast to all waiters.
			s.completionCond.Broadcast()

		case command.EventCancel:
			s.state.Status = StatusIdle
			s.state.Output = nil
			s.state.Error = nil
		}

		s.mu.Unlock()
	}
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

func (s *Server) reload(ctx context.Context, path string) (int64, error) {
	if !s.pathChanged(path) {
		return 0, nil
	}

	// Reconfigure the runner with the new path.
	err := s.runner.Configure(
		command.WithAutoProfile(),
		command.WithPath(path),
	)
	if err != nil {
		return 0, fmt.Errorf("reconfigure runner with path %q: %w", path, err)
	}

	s.currentPath = path

	// Increment request count and run the command with the new path.
	requestNumber := atomic.AddInt64(&s.requestCount, 1)
	go s.runner.RunContext(ctx)

	return requestNumber, nil
}

// waitForCompletion blocks until any command execution completes after the given request number or the context is canceled.
func (s *Server) waitForCompletion(ctx context.Context, requestNumber int64) error {
	if requestNumber == 0 {
		return nil // No reload happened.
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a channel for context cancellation.
	ctxDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(ctxDone)
		s.completionCond.Broadcast() // Wake up the condition variable.
	}()

	// Check if we already have a recent completion.
	for {
		if s.state.Status == StatusCompleted || s.state.Status == StatusError {
			if s.state.RequestCount >= requestNumber {
				return nil
			}
		}

		// Check if context was canceled.
		select {
		case <-ctxDone:
			return fmt.Errorf("wait for completion canceled: %w", ctx.Err())
		default:
		}

		// Wait for the next completion.
		s.completionCond.Wait()
	}
}

// handleListResources handles the list_resources tool call.
func (s *Server) handleListResources(
	ctx context.Context,
	_ *mcp.ServerSession,
	params *mcp.CallToolParamsFor[ListResourcesParams],
) (*mcp.CallToolResultFor[ListResourcesResult], error) {
	startTime := time.Now()

	slog.DebugContext(ctx, "handling list_resources tool call", slog.Any("params", params))

	s.mu.Lock()

	requestNumber, err := s.reload(ctx, params.Arguments.Path)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("reconfigure runner: %w", err)
	}

	s.mu.Unlock()

	// Wait for any completion that occurs after our request was made.
	err = s.waitForCompletion(ctx, requestNumber)
	if err != nil {
		return nil, fmt.Errorf("wait for completion: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := ListResourcesResult{
		Status:    string(s.state.Status),
		Resources: []kube.ResourceMetadata{},
	}

	if s.state.Error != nil {
		result.Error = s.state.Error.Error()
	}

	if s.state.Output == nil {
		return createListResourcesResult(result), nil
	}

	populateResultFromOutput(&result, s.state.Output)

	toolResult := createListResourcesResult(result)

	slog.DebugContext(ctx, "list_resources execution completed",
		slog.String("status", string(s.state.Status)),
		slog.Int("resource_count", len(s.state.Output.Resources)),
		slog.Duration("duration", time.Since(startTime)),
	)

	return toolResult, nil
}

// handleGetResource handles the get_resource tool call.
func (s *Server) handleGetResource(
	ctx context.Context,
	_ *mcp.ServerSession,
	params *mcp.CallToolParamsFor[GetResourceParams],
) (*mcp.CallToolResultFor[GetResourceResult], error) {
	slog.DebugContext(ctx, "handling get_resource tool call", slog.Any("params", params))

	s.mu.Lock()

	requestNumber, err := s.reload(ctx, params.Arguments.Path)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("reconfigure runner: %w", err)
	}

	s.mu.Unlock()

	// Wait for any completion that occurs after our request was made.
	err = s.waitForCompletion(ctx, requestNumber)
	if err != nil {
		return nil, fmt.Errorf("wait for completion: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := GetResourceResult{
		Status: string(s.state.Status),
		Found:  false,
	}

	if s.state.Error != nil {
		result.Error = s.state.Error.Error()
	}

	if s.state.Output == nil {
		return createGetResourceResult(result, params.Arguments), nil
	}

	// Search for the requested resource.
	resource := findResource(s.state.Output.Resources, params.Arguments)
	if resource != nil {
		result.Found = true
		result.Resource = &ResourceDetails{
			Metadata: resource.Object.GetMetadata(),
			YAML:     resource.YAML,
		}
	}

	return createGetResourceResult(result, params.Arguments), nil
}

func (s *Server) Server() *mcp.Server {
	return s.server
}

func (s *Server) Close() {
	close(s.eventCh)
	// Wake up any waiting goroutines before closing.
	s.mu.Lock()
	s.completionCond.Broadcast()
	s.mu.Unlock()
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
