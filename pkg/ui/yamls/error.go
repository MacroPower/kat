package yamls

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/macropower/kat/pkg/ui/ansis"
)

// ErrorPosition represents an error position within the content.
type ErrorPosition struct {
	Line  int // 0-based line number.
	Start int // 0-based character position within the line.
	End   int // 0-based character position within the line (exclusive).
}

// ErrorHighlighter handles error-specific highlighting via [*ansis.StyleEditor].
type ErrorHighlighter struct {
	errorStyle lipgloss.Style
}

// NewErrorHighlighter creates a new [ErrorHighlighter].
func NewErrorHighlighter(errorStyle lipgloss.Style) *ErrorHighlighter {
	return &ErrorHighlighter{
		errorStyle: errorStyle,
	}
}

// ApplyErrorHighlights applies error highlighting to content that already has chroma styling.
// It converts [ErrorPosition] slices to [ansis.StyleRange] slices and delegates to the [*ansis.StyleEditor].
func (eh *ErrorHighlighter) ApplyErrorHighlights(text string, errors []ErrorPosition) string {
	if len(errors) == 0 {
		return text
	}

	lineRanges := eh.convertErrorsToStyleRanges(errors)

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

// convertErrorsToStyleRanges converts [ErrorPosition] slices to [ansis.StyleRange] slices organized by line.
func (eh *ErrorHighlighter) convertErrorsToStyleRanges(errors []ErrorPosition) map[int][]ansis.StyleRange {
	lineRanges := map[int][]ansis.StyleRange{}

	for _, err := range errors {
		styleRange := ansis.StyleRange{
			Start:    err.Start,
			End:      err.End,
			Style:    eh.errorStyle,
			Priority: 4, // Higher priority than diffs and search highlights to ensure errors show on top.
		}

		lineRanges[err.Line] = append(lineRanges[err.Line], styleRange)
	}

	return lineRanges
}
