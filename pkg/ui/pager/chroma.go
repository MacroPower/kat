package pager

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"github.com/MacroPower/kat/pkg/ui/themes"
)

const (
	wrapOnCharacters = " /-"
)

// ChromaRenderer handles rendering content with chroma styling.
type ChromaRenderer struct {
	theme               *themes.Theme
	lexer               chroma.Lexer
	formatter           chroma.Formatter
	style               *chroma.Style
	lineNumbersDisabled bool
}

// NewChromaRenderer creates a new ChromaRenderer.
func NewChromaRenderer(theme *themes.Theme, lineNumbersDisabled bool) *ChromaRenderer {
	lexer := lexers.Get("YAML")
	lexer = chroma.Coalesce(lexer)

	formatterName := "noop" // Default to noop formatter.
	switch termenv.ColorProfile() {
	case termenv.TrueColor:
		formatterName = "terminal16m"

	case termenv.ANSI256:
		formatterName = "terminal256"

	case termenv.ANSI:
		formatterName = "terminal8"
	}

	formatter := formatters.Get(formatterName)

	return &ChromaRenderer{
		theme:               theme,
		lexer:               lexer,
		formatter:           formatter,
		style:               theme.ChromaStyle,
		lineNumbersDisabled: lineNumbersDisabled,
	}
}

// RenderContent renders YAML content with chroma styling.
func (gr *ChromaRenderer) RenderContent(yaml string, width int) (string, error) {
	content, err := gr.executeRendering(yaml)
	if err != nil {
		return "", err
	}

	return gr.postProcessContent(content, width), nil
}

// executeRendering performs the actual chroma rendering.
func (gr *ChromaRenderer) executeRendering(yaml string) (string, error) {
	iterator, err := gr.lexer.Tokenise(nil, yaml)
	if err != nil {
		return "", fmt.Errorf("lexer tokenize: %w", err)
	}

	buf := &bytes.Buffer{}
	err = gr.formatter.Format(buf, gr.style, iterator)
	if err != nil {
		return "", fmt.Errorf("format: %w", err)
	}

	return buf.String(), nil
}

// postProcessContent handles post-processing of rendered content.
func (gr *ChromaRenderer) postProcessContent(content string, width int) string {
	content = strings.TrimSpace(content)

	// Trim lines and add line numbers if needed.
	lines := strings.Split(content, "\n")
	var result strings.Builder

	for i, line := range lines {
		if gr.lineNumbersDisabled {
			result.WriteString(gr.formatLine(line, width))
		} else {
			result.WriteString(gr.formatLineWithNumber(line, i+1, width))
		}

		// Don't add an artificial newline after the last split.
		if i+1 < len(lines) {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

func (gr *ChromaRenderer) formatLine(line string, width int) string {
	trunc := lipgloss.NewStyle().MaxWidth(width).Render
	lines := cellbuf.Wrap(line, width, wrapOnCharacters)

	return trunc(lines)
}

// formatLineWithNumber formats a line with line number and truncation.
func (gr *ChromaRenderer) formatLineWithNumber(line string, lineNum, width int) string {
	width = max(0, width-2) // Reserve space for line number and padding.

	trunc := lipgloss.NewStyle().MaxWidth(width).Render
	lineNumberText := fmt.Sprintf("%4d  ", lineNum)

	lines := cellbuf.Wrap(line, width, wrapOnCharacters)

	fmtLines := []string{}
	for i, ln := range strings.Split(lines, "\n") {
		if i == 0 {
			// Add line number only to the first line.
			ln = gr.theme.LineNumberStyle.Render(lineNumberText) + trunc(ln)
		} else {
			// For subsequent lines, just add spaces for alignment.
			ln = gr.theme.LineNumberStyle.Render("   -  ") + trunc(ln)
		}
		fmtLines = append(fmtLines, ln)
	}

	return strings.Join(fmtLines, "\n")
}
