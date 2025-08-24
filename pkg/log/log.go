package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"go.opentelemetry.io/otel/trace"

	charmlog "github.com/charmbracelet/log"
)

type (
	Format string
	Level  string

	contextKey string
)

const (
	FormatJSON   Format = "json"
	FormatLogfmt Format = "logfmt"
	FormatText   Format = "text"

	LevelError Level = "error"
	LevelWarn  Level = "warn"
	LevelInfo  Level = "info"
	LevelDebug Level = "debug"

	loggerContextKey contextKey = "logger"
)

var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrUnknownLogLevel  = errors.New("unknown log level")
	ErrUnknownLogFormat = errors.New("unknown log format")

	AllFormats = []string{
		string(FormatJSON),
		string(FormatLogfmt),
		string(FormatText),
	}
	AllLevels = []string{
		string(LevelError),
		string(LevelWarn),
		string(LevelInfo),
		string(LevelDebug),
	}
)

// CreateHandlerWithStrings creates a [slog.Handler] by strings.
func CreateHandlerWithStrings(w io.Writer, logLevel, logFormat string) (slog.Handler, error) {
	logLvl, err := GetLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	logFmt, err := GetFormat(logFormat)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	return CreateHandler(w, logLvl, logFmt), nil
}

func CreateHandler(w io.Writer, logLvl slog.Level, logFmt Format) slog.Handler {
	switch logFmt {
	case FormatJSON:
		return slog.NewJSONHandler(w, &slog.HandlerOptions{
			AddSource: true,
			Level:     logLvl,
		})

	case FormatLogfmt:
		return slog.NewTextHandler(w, &slog.HandlerOptions{
			AddSource: true,
			Level:     logLvl,
		})

	case FormatText:
		return newCharmLogHandler(w, logLvl)
	}

	return nil
}

func GetLevel(level string) (slog.Level, error) {
	switch Level(strings.ToLower(level)) {
	case LevelError:
		return slog.LevelError, nil
	case LevelWarn, "warning":
		return slog.LevelWarn, nil
	case LevelInfo:
		return slog.LevelInfo, nil
	case LevelDebug:
		return slog.LevelDebug, nil
	}

	return 0, ErrUnknownLogLevel
}

func GetFormat(format string) (Format, error) {
	logFmt := Format(strings.ToLower(format))
	if slices.Contains([]Format{FormatJSON, FormatLogfmt, FormatText}, logFmt) {
		return logFmt, nil
	}

	return "", ErrUnknownLogFormat
}

func newCharmLogHandler(w io.Writer, level slog.Level) slog.Handler {
	//nolint:gosec // G115: input from GetLevel.
	lvl := int32(level)

	logger := charmlog.NewWithOptions(w, charmlog.Options{
		Level:           charmlog.Level(lvl),
		Formatter:       charmlog.TextFormatter,
		ReportTimestamp: true,
		ReportCaller:    true,
		TimeFormat:      time.StampMilli,
	})
	logger.SetColorProfile(termenv.ColorProfile())

	return logger
}

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
