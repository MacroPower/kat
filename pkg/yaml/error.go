package yaml

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

func NewPathBuilder() *yaml.PathBuilder {
	// Use the goccy/go-yaml PathBuilder to create a new YAMLPath.
	return &yaml.PathBuilder{}
}

type ErrorWrapper struct {
	Opts []ErrorOpt
}

func NewErrorWrapper(opts ...ErrorOpt) *ErrorWrapper {
	return &ErrorWrapper{
		Opts: opts,
	}
}

// Wrap wraps an error with additional context for [Error]s.
// If the error isn't an [Error], it returns the original error unmodified.
func (ew *ErrorWrapper) Wrap(err error, opts ...ErrorOpt) error {
	if err == nil {
		return nil
	}

	var yamlErr *Error
	if errors.As(err, &yamlErr) {
		for _, opt := range ew.Opts {
			opt(yamlErr)
		}

		for _, opt := range opts {
			opt(yamlErr)
		}

		return yamlErr
	}

	return err
}

// Error represents a YAML error. It includes the original error, and the
// [*token.Token] where the error occurred.
type Error struct {
	Err         error
	Path        *yaml.Path
	Token       *token.Token
	Theme       *theme.Theme
	Formatter   string
	Source      []byte
	SourceLines int // Number of lines to show around the error in the source.
}

func NewError(err error, opts ...ErrorOpt) *Error {
	e := &Error{
		Err:         err,
		SourceLines: 4,
		Theme:       theme.Default,
	}
	for _, opt := range opts {
		opt(e)
	}

	return e
}

type ErrorOpt func(e *Error)

func WithSourceLines(lines int) ErrorOpt {
	return func(e *Error) {
		e.SourceLines = lines
	}
}

func WithPath(path *yaml.Path) ErrorOpt {
	return func(e *Error) {
		e.Path = path
	}
}

func WithToken(tk *token.Token) ErrorOpt {
	return func(e *Error) {
		e.Token = tk
	}
}

func WithTheme(t *theme.Theme) ErrorOpt {
	return func(e *Error) {
		e.Theme = t
	}
}

func WithFormatter(formatter string) ErrorOpt {
	return func(e *Error) {
		e.Formatter = formatter
	}
}

func WithSource(source []byte) ErrorOpt {
	return func(e *Error) {
		e.Source = source
	}
}

func (e Error) Error() string {
	if e.Err == nil {
		return ""
	}
	if e.Path == nil && e.Token == nil {
		return e.Err.Error()
	}

	errMsg, srcErr := e.annotateSource(e.Source, e.Path)
	if srcErr != nil {
		slog.Warn("failed to annotate config with error",
			slog.String("path", e.Path.String()),
			slog.Any("error", srcErr),
		)
		// If we can't annotate the source, just return the error without it.
		return fmt.Sprintf("error at %s: %v", e.Path.String(), e.Err)
	}

	return errMsg
}

// Replaces [github.com/goccy/go-yaml.Path.AnnotateSource] to render the source with Chroma.
// Normally it uses [github.com/goccy/go-yaml/printer.Printer].
func (e Error) annotateSource(source []byte, path *yaml.Path) (string, error) {
	var (
		tk  = e.Token
		err error
	)
	if e.Token == nil {
		tk, err = getTokenFromPath(source, path)
		if err != nil {
			return "", fmt.Errorf("get token from path: %w", err)
		}
	}

	errLine, errCol, _, _ := getTokenPosition(tk)
	errMsg := fmt.Sprintf("[%d:%d] %v:", errLine, errCol, e.Err)

	errSource := e.printErrorTokenChroma(tk)
	errSource = lipgloss.NewStyle().
		PaddingTop(1).
		Render(errSource)

	return fmt.Sprintf("%s\n%s", errMsg, errSource), nil
}

func (e Error) printErrorTokenChroma(tk *token.Token) string {
	var pp Printer

	content, initialLineNumber := pp.PrintErrorToken(tk.Clone(), e.SourceLines)

	errStartLine, errStartCol, errEndLine, errEndCol := getTokenPosition(tk)

	cr := yamls.NewChromaRenderer(e.Theme, yamls.WithInitialLineNumber(initialLineNumber))
	if e.Formatter != "" {
		cr.SetFormatter(e.Formatter)
	}

	// Adjust to zero-based index for the error line.
	errStartLine -= initialLineNumber
	errEndLine -= initialLineNumber

	if e.Path != nil {
		// If using a specific path, set the error for the start line only.
		// TODO: Make this work properly (currently the token end position is incorrect).
		cr.SetError(errStartLine, errStartCol, errStartLine, 300)
	} else {
		cr.SetError(errStartLine, errStartCol, errEndLine, errEndCol)
	}

	out, err := cr.RenderContent(content, 0)
	if err != nil {
		return fmt.Sprintf("error rendering token: %v", err)
	}

	return out
}

func getTokenFromPath(source []byte, path *yaml.Path) (*token.Token, error) {
	file, err := parser.ParseBytes(source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse source bytes into ast.File: %w", err)
	}

	node, err := path.FilterFile(file)
	if err != nil {
		return nil, fmt.Errorf("filter from ast.File by YAMLPath: %w", err)
	}

	// Try to find the key token by looking up parent.
	// This is useful because path.FilterFile returns the VALUE node,
	// but for error reporting we want to point to the KEY.
	keyToken := findKeyToken(file, path)
	if keyToken != nil {
		return keyToken, nil
	}

	return node.GetToken(), nil
}

// findKeyToken attempts to find the KEY token for the given path by looking
// in the parent node.
func findKeyToken(file *ast.File, path *yaml.Path) *token.Token {
	pathStr := path.String()

	// Find the last segment and build parent path.
	lastDot := strings.LastIndex(pathStr, ".")
	lastBracket := strings.LastIndex(pathStr, "[")

	if lastDot == -1 && lastBracket == -1 {
		return nil // Root path, no parent.
	}

	if lastDot <= lastBracket {
		// Array index case - no key to find.
		return nil
	}

	parentPathStr := pathStr[:lastDot]
	lastSegment := pathStr[lastDot+1:]

	parentPath, err := yaml.PathString(parentPathStr)
	if err != nil {
		return nil
	}

	parentNode, err := parentPath.FilterFile(file)
	if err != nil {
		return nil
	}

	// Find matching key in parent mapping.
	if mapping, ok := parentNode.(*ast.MappingNode); ok {
		for _, val := range mapping.Values {
			if val.Key.String() == lastSegment {
				return val.Key.GetToken()
			}
		}
	}

	return nil
}

// getTokenPosition returns the start and end positions of the token in the source.
// Returns line and column indices as (startLine, startCol, endLine, endCol).
//
//nolint:revive // Function-result-limit, fine for coordinates.
func getTokenPosition(tk *token.Token) (int, int, int, int) {
	if tk == nil {
		return 0, 0, 0, 0
	}

	startLine := tk.Position.Line
	endLine := startLine
	startCol := tk.Position.Column - 1 // Convert to zero-based index.

	var endCol int
	if tk.Next == nil {
		endCol = len(tk.Origin) + startCol
	} else {
		endLine = tk.Next.Position.Line
		endCol = tk.Next.Position.Column - 1
	}

	return startLine, startCol, endLine, endCol
}
