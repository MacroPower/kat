package pager

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
	"github.com/sahilm/fuzzy"

	"github.com/MacroPower/kat/pkg/ui/themes"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

const (
	wrapOnCharacters = " /-"
)

// MatchPosition represents a search match position within the content.
type MatchPosition struct {
	Line   int // 0-based line number.
	Start  int // 0-based character position within the line.
	End    int // 0-based character position within the line (exclusive).
	Length int // Length of the match.
}

// ChromaRenderer handles rendering content with chroma styling.
type ChromaRenderer struct {
	lexer               chroma.Lexer
	formatter           chroma.Formatter
	theme               *themes.Theme
	style               *chroma.Style
	searchTerm          string
	matches             []MatchPosition
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
	// Find search matches if search term is set before applying any styling.
	if gr.searchTerm != "" {
		gr.findMatches(yaml)
	}

	// First apply chroma syntax highlighting to the original content.
	content, err := gr.executeRendering(yaml)
	if err != nil {
		return "", err
	}

	// Apply search highlighting to the chroma-styled content.
	if gr.searchTerm != "" && len(gr.matches) > 0 {
		content = gr.applySearchHighlightingToStyledContent(content, yaml)
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

// SetSearchTerm sets the search term and clears existing matches.
func (gr *ChromaRenderer) SetSearchTerm(term string) {
	gr.searchTerm = term
	gr.matches = nil
}

// GetMatches returns the current search matches.
func (gr *ChromaRenderer) GetMatches() []MatchPosition {
	return gr.matches
}

// SetFormatter sets the chroma formatter explicitly.
// This is primarily useful for testing.
func (gr *ChromaRenderer) SetFormatter(name string) {
	gr.formatter = formatters.Get(name)
}

// findMatches finds all occurrences of the search term in the content.
func (gr *ChromaRenderer) findMatches(content string) {
	gr.matches = nil

	if gr.searchTerm == "" {
		return
	}

	normalizedTerm, err := yamldoc.Normalize(gr.searchTerm)
	if err != nil {
		slog.Debug("error normalizing search term",
			slog.Any("error", err),
		)
		normalizedTerm = gr.searchTerm
	}

	lines := strings.Split(content, "\n")

	// Find matches line by line for better accuracy.
	for lineNum, line := range lines {
		normalizedLine, err := yamldoc.Normalize(line)
		if err != nil {
			normalizedLine = line
		}

		fuzzyMatches := fuzzy.Find(normalizedTerm, []string{normalizedLine})
		if len(fuzzyMatches) == 0 {
			continue
		}

		// Convert fuzzy match indexes to line positions.
		match := fuzzyMatches[0]
		for _, matchIdx := range match.MatchedIndexes {
			gr.matches = append(gr.matches, MatchPosition{
				Line:   lineNum,
				Start:  matchIdx,
				End:    matchIdx + 1,
				Length: 1,
			})
		}
	}

	// Group consecutive matches.
	gr.groupConsecutiveMatches()
}

// groupConsecutiveMatches groups consecutive character matches into larger matches.
func (gr *ChromaRenderer) groupConsecutiveMatches() {
	if len(gr.matches) == 0 {
		return
	}

	var grouped []MatchPosition
	current := gr.matches[0]

	for i := 1; i < len(gr.matches); i++ {
		match := gr.matches[i]

		// If this match is consecutive to the current one on the same line.
		if match.Line == current.Line && match.Start == current.End {
			// Extend the current match.
			current.End = match.End
			current.Length = current.End - current.Start
		} else {
			// Save the current match and start a new one.
			grouped = append(grouped, current)
			current = match
		}
	}

	// Don't forget the last match.
	grouped = append(grouped, current)
	gr.matches = grouped
}

// getLineMatches returns all matches for a specific line.
func (gr *ChromaRenderer) getLineMatches(lineNum int) []MatchPosition {
	var lineMatches []MatchPosition
	for _, match := range gr.matches {
		if match.Line == lineNum {
			lineMatches = append(lineMatches, match)
		}
	}

	return lineMatches
}

// applySearchHighlightingToStyledContent applies search highlighting to content that already has chroma styling.
func (gr *ChromaRenderer) applySearchHighlightingToStyledContent(styledContent, originalContent string) string {
	if gr.searchTerm == "" || len(gr.matches) == 0 {
		return styledContent
	}

	styledLines := strings.Split(styledContent, "\n")
	originalLines := strings.Split(originalContent, "\n")
	var result strings.Builder

	for i, styledLine := range styledLines {
		var originalLine string
		if i < len(originalLines) {
			originalLine = originalLines[i]
		}

		highlightedLine := gr.applySearchToStyledLine(styledLine, originalLine, i)
		result.WriteString(highlightedLine)

		// Don't add an artificial newline after the last line.
		if i+1 < len(styledLines) {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// applySearchToStyledLine applies search highlighting to a styled line by finding the original text positions.
func (gr *ChromaRenderer) applySearchToStyledLine(styledLine, originalLine string, lineNum int) string {
	lineMatches := gr.getLineMatches(lineNum)
	if len(lineMatches) == 0 {
		return styledLine
	}

	// For each match, we need to find where that text appears in the styled line
	// and wrap it with our highlight style.
	result := styledLine

	// Process matches in reverse order to avoid offset issues when inserting.
	for i := len(lineMatches) - 1; i >= 0; i-- {
		match := lineMatches[i]
		if match.Start >= 0 && match.End <= len(originalLine) {
			matchText := originalLine[match.Start:match.End]
			highlightStyle := gr.theme.SelectedStyle.Underline(true).Bold(true)
			highlightedText := highlightStyle.Render(matchText)

			// Replace the first occurrence of the match text in the styled line.
			// This is a simple approach that works for most cases.
			result = strings.Replace(result, matchText, highlightedText, 1)
		}
	}

	return result
}
