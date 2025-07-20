package yaml

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/lipgloss"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

func NewPathBuilder() *yaml.PathBuilder {
	// Use the goccy/go-yaml PathBuilder to create a new YAMLPath.
	return &yaml.PathBuilder{}
}

func AddErrorContext(err error, source []byte, t *theme.Theme) error {
	var yamlErr *Error
	if errors.As(err, &yamlErr) {
		yamlErr.Theme = t
		yamlErr.Source = source
		yamlErr.SourceLines = 4 // Default to showing 4 lines around the error.

		return yamlErr
	}

	return err
}

type Error struct {
	Err         error
	Path        *yaml.Path
	Token       *token.Token
	Theme       *theme.Theme
	Formatter   string
	Source      []byte
	SourceLines int // Number of lines to show around the error in the source.
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

func (e Error) getTokenFromPath(source []byte, path *yaml.Path) (*token.Token, error) {
	file, err := parser.ParseBytes(source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse source bytes into ast.File: %w", err)
	}

	node, err := path.FilterFile(file)
	if err != nil {
		return nil, fmt.Errorf("filter from ast.File by YAMLPath: %w", err)
	}

	return node.GetToken(), nil
}

// Replaces [github.com/goccy/go-yaml.Path.AnnotateSource] to render the source with Chroma.
// Normally it uses [github.com/goccy/go-yaml/printer.Printer].
func (e Error) annotateSource(source []byte, path *yaml.Path) (string, error) {
	var (
		tk  = e.Token
		err error
	)
	if e.Token == nil {
		tk, err = e.getTokenFromPath(source, path)
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

	if errStartLine == errEndLine {
		cr.SetError(errStartLine, errStartCol, errEndCol)
	} else if e.Path != nil {
		// If using a specific path, set the error for the start line only.
		cr.SetError(errStartLine, errStartCol, 80)
	} else {
		for i := errStartLine; i <= errEndLine; i++ {
			// TODO: Move this to SetError.
			switch i {
			case errStartLine:
				cr.SetError(i, errStartCol, 80)
			case errEndLine:
				cr.SetError(i, 0, errEndCol)
			default:
				cr.SetError(i, 0, 80) // Full line error for lines in between.
			}
		}
	}

	out, err := cr.RenderContent(content, 0)
	if err != nil {
		return fmt.Sprintf("error rendering token: %v", err)
	}

	return out
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
