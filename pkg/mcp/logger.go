package mcp

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/macropower/kat/pkg/log"
)

// TracedToolHandler wraps an MCP ToolHandlerFor with automatic tracing and logging.
type TracedToolHandler[In, Out any] func(
	context.Context,
	*mcp.ServerSession,
	*mcp.CallToolParamsFor[In],
) (*mcp.CallToolResultFor[Out], error)

// WithTracing wraps a TracedToolHandler with automatic OpenTelemetry tracing and structured logging.
// It creates a span for each tool call, adds trace IDs to logs, and records errors on spans.
func WithTracing[In, Out any](
	tracer trace.Tracer,
	handler TracedToolHandler[In, Out],
) mcp.ToolHandlerFor[In, Out] {
	return func(
		ctx context.Context,
		session *mcp.ServerSession,
		params *mcp.CallToolParamsFor[In],
	) (*mcp.CallToolResultFor[Out], error) {
		name := params.Name

		// Start a new span for this tool call.
		ctx, span := tracer.Start(ctx, name)
		defer span.End()

		logger := log.WithContext(ctx)

		// Log the start of the tool call.
		logger.DebugContext(ctx, "handling tool call",
			slog.String("name", name),
			slog.Any("progress_token", params.GetProgressToken()),
			slog.Any("args", params.Arguments),
		)

		// Call the actual handler.
		result, err := handler(ctx, session, params)

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

		return result, err
	}
}
