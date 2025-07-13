package yamls

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

	"github.com/macropower/kat/pkg/ui/theme"
)

const (
	wrapOnCharacters = " /-"
)

// ChromaRenderer handles rendering content with chroma styling.
type ChromaRenderer struct {
	lexer                chroma.Lexer
	formatter            chroma.Formatter
	theme                *theme.Theme
	style                *chroma.Style
	searchHighlighter    *SearchHighlighter
	diffHighlighter      *DiffHighlighter
	differ               *Differ
	searchTerm           string
	matches              []MatchPosition
	currentSelectedMatch int // Index of the currently selected match (-1 if none selected).
	lineNumbersDisabled  bool
}

// NewChromaRenderer creates a new [ChromaRenderer].
func NewChromaRenderer(t *theme.Theme, lineNumbersDisabled bool) *ChromaRenderer {
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

	highlightStyle := t.SelectedStyle.Underline(true).Bold(true)
	selectedHighlightStyle := t.LogoStyle.Bold(true)

	insertedStyle := t.InsertedStyle
	deletedStyle := t.DeletedStyle
	editedStyle := t.InsertedStyle

	return &ChromaRenderer{
		currentSelectedMatch: -1,
		theme:                t,
		lexer:                lexer,
		formatter:            formatter,
		style:                t.ChromaStyle,
		lineNumbersDisabled:  lineNumbersDisabled,
		searchHighlighter:    NewSearchHighlighter(highlightStyle, selectedHighlightStyle),
		diffHighlighter:      NewDiffHighlighter(insertedStyle, deletedStyle, editedStyle),
		differ:               NewDiffer(),
	}
}

// RenderContent renders YAML content with chroma styling.
func (gr *ChromaRenderer) RenderContent(yaml string, width int) (string, error) {
	// Find search matches if search term is set before applying any styling.
	if gr.searchTerm != "" {
		gr.findMatches(yaml)
	}

	gr.differ.SetInitialContent(yaml)

	// Set original content and find diffs using the DiffHighlighter.
	diffs := gr.differ.FindAndCacheDiffs(yaml)

	// First apply chroma syntax highlighting to the original content.
	content, err := gr.executeRendering(yaml)
	if err != nil {
		return "", err
	}

	// Apply diff highlighting to the chroma-styled content.
	content = gr.diffHighlighter.ApplyDiffHighlights(content, diffs)

	// Apply search highlighting to the chroma-styled content using the highlighter.
	if gr.searchTerm != "" && len(gr.matches) > 0 {
		content = gr.searchHighlighter.ApplyHighlights(content, gr.matches, gr.currentSelectedMatch)
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
	gr.SetCurrentSelectedMatch(-1)
}

// SetCurrentSelectedMatch sets the index of the currently selected match.
func (gr *ChromaRenderer) SetCurrentSelectedMatch(index int) {
	gr.currentSelectedMatch = index
}

// GetCurrentSelectedMatch returns the index of the currently selected match.
func (gr *ChromaRenderer) GetCurrentSelectedMatch() int {
	return gr.currentSelectedMatch
}

// GetMatches returns the current search matches.
func (gr *ChromaRenderer) GetMatches() []MatchPosition {
	return gr.matches
}

func (gr *ChromaRenderer) ClearDiffs() {
	gr.differ.ClearDiffs()
}

// Unload clears the current state of the renderer.
func (gr *ChromaRenderer) Unload() {
	gr.differ.Unload()
	gr.SetSearchTerm("")
}

// SetFormatter sets the chroma formatter explicitly.
// This is primarily useful for testing.
func (gr *ChromaRenderer) SetFormatter(name string) {
	gr.formatter = formatters.Get(name)
}

// FindMatchesInContent finds matches in the given content and stores them.
// This is useful when you need to find matches immediately rather than waiting for the next render.
func (gr *ChromaRenderer) FindMatchesInContent(content string) {
	if gr.searchTerm != "" {
		gr.findMatches(content)
	}
}

// findMatches finds all occurrences of the search term in the content.
func (gr *ChromaRenderer) findMatches(content string) {
	gr.matches = nil

	if gr.searchTerm == "" {
		return
	}

	normalizedTerm, err := Normalize(gr.searchTerm)
	if err != nil {
		slog.Debug("error normalizing search term",
			slog.Any("error", err),
		)

		normalizedTerm = gr.searchTerm
	}

	lines := strings.Split(content, "\n")

	// Find matches line by line.
	for lineNum, line := range lines {
		normalizedLine, err := Normalize(line)
		if err != nil {
			normalizedLine = line
		}

		gr.findSubstringMatches(normalizedLine, normalizedTerm, lineNum)
	}
}

// findSubstringMatches finds all substring occurrences.
func (gr *ChromaRenderer) findSubstringMatches(line, term string, lineNum int) {
	if term == "" {
		return
	}

	// Convert to lowercase for case-insensitive search (e.g. ignorecase).
	lowerLine := strings.ToLower(line)
	lowerTerm := strings.ToLower(term)

	lineRunes := []rune(line)
	searchIndex := 0

	for searchIndex < len(lowerLine) {
		// Find the next occurrence.
		matchStart := strings.Index(lowerLine[searchIndex:], lowerTerm)
		if matchStart == -1 {
			break // No more matches.
		}

		// Convert byte positions to rune positions.
		absoluteByteStart := searchIndex + matchStart
		absoluteByteEnd := absoluteByteStart + len(lowerTerm)

		runeStart := gr.byteIndexToRuneIndex(line, absoluteByteStart)
		runeEnd := gr.byteIndexToRuneIndex(line, absoluteByteEnd)

		// Handle edge case where the term ends at the line boundary.
		if runeEnd == -1 {
			runeEnd = len(lineRunes)
		}

		if runeStart >= 0 && runeEnd > runeStart {
			gr.matches = append(gr.matches, MatchPosition{
				Line:  lineNum,
				Start: runeStart,
				End:   runeEnd,
			})
		}

		// Move past this match to find the next one.
		searchIndex = absoluteByteStart + 1
	}
}

// byteIndexToRuneIndex converts a byte index to a rune index.
func (gr *ChromaRenderer) byteIndexToRuneIndex(s string, byteIdx int) int {
	if byteIdx >= len(s) {
		return -1
	}

	runeIdx := 0
	for i := range s {
		if i == byteIdx {
			return runeIdx
		}
		if i > byteIdx {
			return -1
		}

		runeIdx++
	}

	return runeIdx
}
