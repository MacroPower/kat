package log

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatLogfmt Format = "logfmt"
)

var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrUnknownLogLevel  = errors.New("unknown log level")
	ErrUnknownLogFormat = errors.New("unknown log format")
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
	}

	return nil
}

func GetLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "error":
		return slog.LevelError, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	}

	return 0, ErrUnknownLogLevel
}

func GetFormat(format string) (Format, error) {
	logFmt := Format(strings.ToLower(format))
	if slices.Contains([]Format{FormatJSON, FormatLogfmt}, logFmt) {
		return logFmt, nil
	}

	return "", ErrUnknownLogFormat
}
