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
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"github.com/macropower/kat/pkg/ui/theme"
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
	lexer                  chroma.Lexer
	formatter              chroma.Formatter
	highlightStyle         lipgloss.Style
	selectedHighlightStyle lipgloss.Style
	theme                  *theme.Theme
	style                  *chroma.Style
	searchTerm             string
	matches                []MatchPosition
	currentSelectedMatch   int // Index of the currently selected match (-1 if none selected).
	lineNumbersDisabled    bool
}

// NewChromaRenderer creates a new ChromaRenderer.
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

	return &ChromaRenderer{
		theme:                  t,
		lexer:                  lexer,
		formatter:              formatter,
		style:                  t.ChromaStyle,
		highlightStyle:         highlightStyle,
		selectedHighlightStyle: selectedHighlightStyle,
		currentSelectedMatch:   -1,
		lineNumbersDisabled:    lineNumbersDisabled,
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
		content = gr.applySearchHighlightingToStyledContent(content)
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
	gr.currentSelectedMatch = -1
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
				Line:   lineNum,
				Start:  runeStart,
				End:    runeEnd,
				Length: runeEnd - runeStart,
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
func (gr *ChromaRenderer) applySearchHighlightingToStyledContent(styledContent string) string {
	if gr.searchTerm == "" || len(gr.matches) == 0 {
		return styledContent
	}

	styledLines := strings.Split(styledContent, "\n")
	var result strings.Builder

	for i, styledLine := range styledLines {
		highlightedLine := gr.applySearchToStyledLine(styledLine, i)
		result.WriteString(highlightedLine)

		// Don't add an artificial newline after the last line.
		if i+1 < len(styledLines) {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// applySearchToStyledLine applies search highlighting to a styled line by finding the original text positions.
func (gr *ChromaRenderer) applySearchToStyledLine(styledLine string, lineNum int) string {
	lineMatches := gr.getLineMatches(lineNum)
	if len(lineMatches) == 0 {
		return styledLine
	}

	// Use ANSI-aware approach to preserve styling.
	return gr.highlightMatchesInStyledText(styledLine, lineMatches)
}

// highlightMatchesInStyledText applies search highlighting while preserving ANSI sequences.
func (gr *ChromaRenderer) highlightMatchesInStyledText(styledText string, matches []MatchPosition) string {
	if len(matches) == 0 {
		return styledText
	}

	// Strip ANSI codes to get plain text positions.
	plainText := ansi.Strip(styledText)

	// Parse the styled text and rebuild it with highlights.
	return gr.rebuildStyledTextWithHighlights(styledText, plainText, matches)
}

// rebuildStyledTextWithHighlights rebuilds styled text with search highlights applied.
func (gr *ChromaRenderer) rebuildStyledTextWithHighlights(styledText, plainText string, matches []MatchPosition) string {
	// Create a map for quick lookup of which positions should be highlighted and which match they belong to.
	highlightMap := make(map[int]int) // Map of position -> global match index (-1 for regular highlight).
	for _, match := range matches {
		// Find the global index of this match in the overall matches slice.
		globalMatchIdx := gr.getMatchIndex(match)
		for i := match.Start; i < match.End; i++ {
			highlightMap[i] = globalMatchIdx // Store the global index (or -1 if not found).
		}
	}

	var result strings.Builder

	plainIdx := 0
	inEscape := false
	var escapeBuffer strings.Builder
	currentStyle := ""

	for _, r := range styledText {
		switch {
		case r == '\x1b':
			inEscape = true
			escapeBuffer.Reset()
			escapeBuffer.WriteRune(r)
		case inEscape:
			escapeBuffer.WriteRune(r)
			if r == 'm' {
				inEscape = false
				escapeSeq := escapeBuffer.String()
				result.WriteString(escapeSeq)

				// Update current style context.
				if escapeSeq != "\x1b[0m" {
					currentStyle = escapeSeq
				} else {
					currentStyle = ""
				}
			}
		default:
			// This is a regular character.
			if globalMatchIdx, shouldHighlight := highlightMap[plainIdx]; shouldHighlight && plainIdx < len([]rune(plainText)) {
				// Determine which style to use based on whether this match is selected.
				var highlightStyle lipgloss.Style
				if globalMatchIdx >= 0 && gr.isMatchSelected(globalMatchIdx) {
					highlightStyle = gr.selectedHighlightStyle
				} else {
					// Use regular highlight style for all matches, including those with globalMatchIdx = -1.
					highlightStyle = gr.highlightStyle
				}

				// Apply highlight to this character and restore styling afterward.
				highlightText := highlightStyle.Render(string(r))
				result.WriteString(highlightText)
				// Restore the current styling context if there was one.
				if currentStyle != "" {
					result.WriteString(currentStyle)
				}
			} else {
				// Normal character.
				result.WriteRune(r)
			}

			plainIdx++
		}
	}

	return result.String()
}

// isMatchSelected returns true if the given match index corresponds to the currently selected match.
func (gr *ChromaRenderer) isMatchSelected(matchIndex int) bool {
	return gr.currentSelectedMatch >= 0 && matchIndex == gr.currentSelectedMatch
}

// getMatchIndex returns the index of a match position in the matches slice.
func (gr *ChromaRenderer) getMatchIndex(match MatchPosition) int {
	for i, m := range gr.matches {
		if m.Line == match.Line && m.Start == match.Start && m.End == match.End {
			return i
		}
	}
	// If not found, this means it's still a valid match but we couldn't find its global index.
	// This can happen if the match was created during line processing but doesn't exactly
	// match the original due to normalization differences.
	return -1
}
