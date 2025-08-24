package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

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

// NewServer creates a new [Server].
func NewServer(address string, runner CommandRunner, initialPath string) (*Server, error) {
	impl := &mcp.Implementation{
		Name:    name,
		Title:   name,
		Version: version.GetVersion(),
	}

	opts := &mcp.ServerOptions{
		Instructions: instructions,
	}

	s := &Server{
		address:     address,
		server:      mcp.NewServer(impl, opts),
		runner:      runner,
		eventCh:     make(chan command.Event, 100),
		currentPath: initialPath,
		state:       ExecutionState{},
		tracer:      otel.Tracer("mcp-server"),
	}

	s.completionCond = sync.NewCond(&s.mu)

	runner.Subscribe(s.eventCh)

	// Register tools with the MCP server.
	s.server.AddTool(
		mcp.ToolFor(newToolListResources(), withTracing(s.tracer, s.handleListResources)),
	)
	s.server.AddTool(
		mcp.ToolFor(newToolGetResource(), withTracing(s.tracer, s.handleGetResource)),
	)

	// Start event processing.
	go s.processEvents()

	return s, nil
}

// handleListResources handles the list_resources tool call.
func (s *Server) handleListResources(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	params ListResourcesParams,
) (*mcp.CallToolResult, ListResourcesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.reload(ctx, params.Path)
	if err != nil {
		return nil, ListResourcesResult{}, fmt.Errorf("reconfigure runner: %w", err)
	}

	result := ListResourcesResult{
		Resources: []kube.ResourceMetadata{},
	}

	if s.state.Output.Error != nil {
		result.Error = s.state.Output.Error.Error()
	}

	populateResultFromOutput(&result, s.state.Output)

	s.runner.SendEvent(command.NewEventListResources(ctx))

	result.Message = fmt.Sprintf("Found %d Kubernetes resources.", result.ResourceCount)

	return createListResourcesResult(result), result, nil
}

// handleGetResource handles the get_resource tool call.
func (s *Server) handleGetResource(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	params GetResourceParams,
) (*mcp.CallToolResult, GetResourceResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.reload(ctx, params.Path)
	if err != nil {
		return nil, GetResourceResult{}, fmt.Errorf("reconfigure runner: %w", err)
	}

	result := GetResourceResult{
		Found: false,
	}

	if s.state.Output.Error != nil {
		result.Error = s.state.Output.Error.Error()
	}

	// Search for the requested resource.
	resource := findResource(s.state.Output.Resources, params)
	if resource != nil {
		result.Found = true
		result.Resource = &ResourceDetails{
			Metadata: resource.Object.GetMetadata(),
			YAML:     resource.YAML,
		}

		// Send event to open the resource in the pager.
		s.runner.SendEvent(command.NewEventOpenResource(ctx, *resource))
	}

	result.Message = formatResourceMessage(result, params)

	return createGetResourceResult(result), result, nil
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
	err := s.server.Run(ctx, &mcp.StdioTransport{})
	if err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
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

// TracedToolHandler wraps an MCP ToolHandlerFor with automatic tracing and logging.
type TracedToolHandler[In, Out any] func(
	context.Context,
	*mcp.CallToolRequest,
	In,
) (*mcp.CallToolResult, Out, error)

// withTracing wraps a TracedToolHandler with automatic OpenTelemetry tracing and structured logging.
// It creates a span for each tool call, adds trace IDs to logs, and records errors on spans.
func withTracing[In, Out any](
	tracer trace.Tracer,
	handler TracedToolHandler[In, Out],
) mcp.ToolHandlerFor[In, Out] {
	return func(
		ctx context.Context,
		req *mcp.CallToolRequest,
		input In,
	) (*mcp.CallToolResult, Out, error) {
		name := req.Params.Name

		// Start a new span for this tool call.
		ctx, span := tracer.Start(ctx, name)
		defer span.End()

		logger := log.WithContext(ctx)

		// Log the start of the tool call.
		logger.DebugContext(ctx, "handling tool call",
			slog.String("name", name),
			slog.Any("args", input),
		)

		// Call the actual handler.
		result, output, err := handler(ctx, req, input)

		// Handle the result.
		if err != nil {
			logger.ErrorContext(ctx, "tool call failed",
				slog.String("name", name),
				slog.Any("error", err),
			)
			span.RecordError(err)
		} else {
			logger.DebugContext(ctx, "tool call completed successfully")
		}

		return result, output, err
	}
}
