package ansis

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// StyleRange represents a range of text that should have a specific style applied.
type StyleRange struct {
	Style    lipgloss.Style // The style to apply to this range.
	Start    int            // 0-based character position within the line (inclusive).
	End      int            // 0-based character position within the line (exclusive).
	Priority int            // Priority for overlapping ranges (higher priority wins).
}

// StyleEditor handles applying style transformations to ANSI-styled text.
// It preserves existing ANSI styling while adding new styling to specific
// character ranges. It can be used for search highlighting, error marking,
// syntax emphasis, or any other inline style modifications.
type StyleEditor struct{}

// NewStyleEditor creates a new [StyleEditor].
func NewStyleEditor() *StyleEditor {
	return &StyleEditor{}
}

// ApplyStyles applies style ranges to ANSI-styled text while preserving existing styling.
// The ranges are applied in priority order, with higher priority ranges overriding lower ones.
func (ise *StyleEditor) ApplyStyles(text string, ranges []StyleRange) string {
	if len(ranges) == 0 {
		return text
	}

	// Sort ranges by priority (highest first), then by start position.
	sortedRanges := make([]StyleRange, len(ranges))
	copy(sortedRanges, ranges)
	sortRangesByPriority(sortedRanges)

	// Strip ANSI codes to get plain text positions.
	plainText := ansi.Strip(text)

	// Create a style map for quick lookup.
	styleMap := createStyleMap(sortedRanges, len([]rune(plainText)))

	// Rebuild the text with both original and new styling.
	return rebuildStyledTextWithRanges(text, plainText, styleMap)
}

// sortRangesByPriority sorts ranges by priority (highest first), then by start position.
func sortRangesByPriority(ranges []StyleRange) {
	slices.SortFunc(ranges, func(a, b StyleRange) int {
		// Sort by priority first (higher priority first).
		if a.Priority != b.Priority {
			return b.Priority - a.Priority
		}
		// If same priority, sort by start position.
		return a.Start - b.Start
	})
}

// createStyleMap creates a map of character positions to their highest-priority styles.
func createStyleMap(sortedRanges []StyleRange, textLength int) map[int]lipgloss.Style {
	styleMap := make(map[int]lipgloss.Style)

	// Apply ranges in reverse priority order so higher priority overwrites lower.
	for i := len(sortedRanges) - 1; i >= 0; i-- {
		r := sortedRanges[i]
		for pos := r.Start; pos < r.End && pos < textLength; pos++ {
			styleMap[pos] = r.Style
		}
	}

	return styleMap
}

// rebuildStyledTextWithRanges rebuilds styled text with new style ranges applied.
// It groups consecutive characters with the same style to reduce output size.
func rebuildStyledTextWithRanges(styledText, plainText string, styleMap map[int]lipgloss.Style) string {
	var result strings.Builder

	plainRunes := []rune(plainText)
	styledRunes := []rune(styledText)

	plainIdx := 0
	styledIdx := 0
	inEscape := false

	var escapeBuffer strings.Builder

	currentStyle := ""

	for styledIdx < len(styledRunes) {
		r := styledRunes[styledIdx]
		switch {
		case r == '\x1b':
			inEscape = true

			escapeBuffer.Reset()
			escapeBuffer.WriteRune(r)

			styledIdx++

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

			styledIdx++

		default:
			// This is a regular character.
			if style, hasStyle := styleMap[plainIdx]; hasStyle && plainIdx < len(plainRunes) {
				// Find consecutive characters with the same style.
				groupStart := plainIdx
				groupEnd := plainIdx + 1

				// Look ahead for consecutive characters with the same style.
				for groupEnd < len(plainRunes) {
					nextStyle, hasNext := styleMap[groupEnd]
					if !hasNext || !StylesEqual(style, nextStyle) {
						break
					}

					groupEnd++
				}

				// Apply the style to the entire group from plain text.
				groupText := string(plainRunes[groupStart:groupEnd])
				styledText := style.Render(groupText)
				result.WriteString(styledText)

				// Restore the current styling context if there was one.
				if currentStyle != "" {
					result.WriteString(currentStyle)
				}

				// Skip ahead in both indices to avoid processing the same characters again.
				charactersProcessed := groupEnd - groupStart
				plainIdx = groupEnd

				// For styledIdx, we need to advance past the characters we just processed
				// from the styled text, accounting for the fact that we may have ANSI codes.
				for i := 0; i < charactersProcessed && styledIdx < len(styledRunes); {
					if styledRunes[styledIdx] == '\x1b' {
						// Skip over ANSI escape sequence.
						styledIdx = skipAnsiSequence(styledRunes, styledIdx)
					} else {
						// Regular character.
						styledIdx++
						i++
					}
				}
			} else {
				// Normal character - no additional styling.
				result.WriteRune(r)

				plainIdx++
				styledIdx++
			}
		}
	}

	return result.String()
}

// skipAnsiSequence skips over an ANSI escape sequence starting at the given index.
// It returns the new index position after the sequence.
func skipAnsiSequence(runes []rune, startIdx int) int {
	idx := startIdx
	for idx < len(runes) && runes[idx] != 'm' {
		idx++
	}
	if idx < len(runes) {
		idx++ // Skip the 'm'.
	}

	return idx
}

// StylesEqual compares two [lipgloss.Style] objects for equality.
// This is a simple comparison that checks if the rendered output would be the same.
func StylesEqual(a, b lipgloss.Style) bool {
	// Compare by rendering a test character and checking if the output is identical.
	return a.Render("x") == b.Render("x")
}
