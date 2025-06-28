package log

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"

	charmlog "github.com/charmbracelet/log"
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatLogfmt Format = "logfmt"
	FormatText   Format = "text"
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
	case FormatText:
		return newCharmLogHandler(w, logLvl)
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
	if isatty.IsTerminal(os.Stdout.Fd()) {
		logger.SetColorProfile(termenv.ColorProfile())
	}

	return logger
}
