package yamls

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/macropower/kat/pkg/ui/ansis"
)

// Normalize text to aid in the filtering process. In particular, we remove
// diacritics, "ö" becomes "o". Title that Mn is the unicode key for nonspacing
// marks.
func Normalize(in string) (string, error) {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, in)
	if err != nil {
		return "", fmt.Errorf("error normalizing: %w", err)
	}

	return out, nil
}

// MatchPosition represents a search match position within the content.
type MatchPosition struct {
	Line  int // 0-based line number.
	Start int // 0-based character position within the line.
	End   int // 0-based character position within the line (exclusive).
}

// SearchHighlighter handles search-specific highlighting via [*ansis.StyleEditor].
type SearchHighlighter struct {
	highlightStyle         lipgloss.Style
	selectedHighlightStyle lipgloss.Style
}

// NewSearchHighlighter creates a new [SearchHighlighter].
func NewSearchHighlighter(highlightStyle, selectedHighlightStyle lipgloss.Style) *SearchHighlighter {
	return &SearchHighlighter{
		highlightStyle:         highlightStyle,
		selectedHighlightStyle: selectedHighlightStyle,
	}
}

// ApplyHighlights applies search highlighting to content that already has chroma styling.
// It converts [MatchPosition] slices to [StyleRange] slices and delegates to the [*ansis.StyleEditor].
func (sh *SearchHighlighter) ApplyHighlights(text string, matches []MatchPosition, selectedMatch int) string {
	if len(matches) == 0 {
		return text
	}

	lineRanges := sh.convertMatchesToStyleRanges(matches, selectedMatch)

	result := []string{}
	for i, line := range strings.Split(text, "\n") {
		if ranges, exists := lineRanges[i]; exists {
			editor := ansis.NewStyleEditor(line)
			result = append(result, editor.ApplyStyles(ranges))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// convertMatchesToStyleRanges converts [MatchPosition] slices to [StyleRange] slices organized by line.
func (sh *SearchHighlighter) convertMatchesToStyleRanges(
	matches []MatchPosition,
	selectedMatch int,
) map[int][]ansis.StyleRange {
	lineRanges := map[int][]ansis.StyleRange{}

	for globalIdx, match := range matches {
		style := sh.highlightStyle
		priority := 1

		// Use selected style for the currently selected match.
		if globalIdx == selectedMatch {
			style = sh.selectedHighlightStyle
			priority = 2 // Higher priority for selected matches.
		}

		styleRange := ansis.StyleRange{
			Start:    match.Start,
			End:      match.End,
			Style:    style,
			Priority: priority,
		}

		lineRanges[match.Line] = append(lineRanges[match.Line], styleRange)
	}

	return lineRanges
}
