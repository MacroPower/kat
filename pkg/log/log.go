package log

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const loggerContextKey contextKey = "logger"

// WithContext returns the default logger with context.
func WithContext(ctx context.Context) *slog.Logger {
	// First check if there's a logger already stored in context.
	if logger, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok {
		return logger
	}

	// Create logger with trace ID if span is available.
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		// Truncate trace ID to first 8 characters for readability.
		if len(traceID) > 8 {
			traceID = traceID[:8]
		}

		return slog.With(slog.String("trace_id", traceID))
	}

	// Fallback: Just return the default logger.
	return slog.Default()
}
