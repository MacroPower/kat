package ansis

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// AnsiTerminator terminates ANSI color/style sequences.
const ansiTerminator = 'm'

// StyleRange represents a range of text that should have a specific style applied.
type StyleRange struct {
	Style    lipgloss.Style // The style to apply to this range.
	Start    int            // 0-based character position within the line (inclusive).
	End      int            // 0-based character position within the line (exclusive).
	Priority int            // Priority for overlapping ranges (higher priority wins).
}

// StyleEditor handles applying style transformations to ANSI-styled text.
// It preserves existing ANSI styling while adding new styling to specific
// character ranges based on plain text positions.
//
// This is particularly useful for:
//
//   - Search result highlighting in colored text
//   - Error marking in syntax-highlighted code
//   - Adding emphasis to specific parts of formatted output
//   - Creating interactive text displays with hover effects
//
// It works by maintaining parallel indices into both the original styled text
// (with ANSI escape sequences) and the plain text (stripped of ANSI). When
// applying new styles, it maps plain text positions to styled text positions
// while preserving the original formatting.
type StyleEditor struct {
	// Original text with any ANSI sequences intact.
	text string

	// Plain text with all ANSI sequences stripped.
	plainText string

	// Maps plain text positions to styles that should be applied.
	styleMap map[int]lipgloss.Style

	// Current ANSI escape sequence context for restoring after styled groups.
	// Example: "\033[31m" when processing red text.
	currentStyle string

	// Output builders for constructing the final result.
	result, escapeBuffer strings.Builder

	// Character arrays for efficient iteration.
	plainRunes, styledRunes []rune

	// Current positions in plain and styled text respectively.
	// These track where we are in the parallel iteration.
	plainIdx, styledIdx int

	// Whether we're currently inside an ANSI escape sequence.
	// Used to handle escape sequences that span multiple characters.
	inEscape bool
}

// NewStyleEditor creates a new [StyleEditor] for the given text.
func NewStyleEditor(text string) *StyleEditor {
	return &StyleEditor{
		text:      text,
		plainText: ansi.Strip(text),
	}
}

// ApplyStyles applies style ranges to the text while preserving existing styling.
// The ranges are applied in priority order, with higher priority ranges overriding lower ones.
func (se *StyleEditor) ApplyStyles(ranges []StyleRange) string {
	// Initialize processing state (reset indices, clear buffers).
	se.initializeState()

	// If no ranges are provided, nothing to do.
	if len(ranges) == 0 {
		return se.text
	}

	// Validate ranges (e.g. Start < End, non-negative positions).
	validateRanges(ranges)

	// Sort ranges by priority (highest first) and then by start position.
	sortedRanges := make([]StyleRange, len(ranges))
	copy(sortedRanges, ranges)
	sortRangesByPriority(sortedRanges)

	// Create a style map (plain text position -> style to apply) for quick lookup.
	se.styleMap = createStyleMap(sortedRanges, ansi.StringWidth(se.plainText))

	// Rebuild the text with both original and new styling.
	return se.process()
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
// Ranges are processed in reverse priority order so that higher priority styles overwrite lower ones.
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

// initializeState initializes the StyleEditor's processing state for a new text processing operation.
func (se *StyleEditor) initializeState() {
	se.plainRunes = []rune(se.plainText)
	se.styledRunes = []rune(se.text)
	se.currentStyle = ""
	se.plainIdx = 0
	se.styledIdx = 0
	se.inEscape = false
	se.result.Reset()
	se.escapeBuffer.Reset()
}

// process rebuilds the styled text with new style ranges applied.
func (se *StyleEditor) process() string {
	for se.styledIdx < len(se.styledRunes) {
		r := se.styledRunes[se.styledIdx]

		switch {
		// ESC character: Start of ANSI escape sequence.
		case r == ansi.ESC:
			se.processEscapeStart()

		// In Escape: Continue processing escape sequence until [ansiTerminator].
		case se.inEscape:
			se.processEscapeSequence(r)

		// Regular character: Apply new styles or copy unchanged.
		default:
			se.processRegularCharacter()
		}
	}

	return se.result.String()
}

// processEscapeStart handles the start of an ANSI escape sequence.
func (se *StyleEditor) processEscapeStart() {
	se.inEscape = true

	se.escapeBuffer.Reset()
	se.escapeBuffer.WriteRune(se.styledRunes[se.styledIdx])

	se.styledIdx++
}

// processEscapeSequence handles characters within an ANSI escape sequence.
//
// Example processing "\033[31m":
//   - Call 1: char='[', escapeBuffer="\033[", inEscape=true
//   - Call 2: char='3', escapeBuffer="\033[3", inEscape=true
//   - Call 3: char='1', escapeBuffer="\033[31", inEscape=true
//   - Call 4: char='m', escapeBuffer="\033[31m", inEscape=false, result+="\033[31m"
func (se *StyleEditor) processEscapeSequence(r rune) {
	se.escapeBuffer.WriteRune(r)

	if r == ansiTerminator {
		se.inEscape = false
		escapeSeq := se.escapeBuffer.String()
		se.result.WriteString(escapeSeq)
		se.updateCurrentStyle(escapeSeq)
	}

	se.styledIdx++
}

// updateCurrentStyle updates the current style context based on the escape sequence.
//
// Tracks the most recent ANSI escape sequence to maintain styling context.
// Reset sequences clear the context, other sequences update it.
//
// Examples:
//   - escapeSeq="\033[31m" (red) -> currentStyle="\033[31m"
//   - escapeSeq="\033[0m" (reset) -> currentStyle=""
//   - escapeSeq="\033[1m" (bold) -> currentStyle="\033[1m"
//
// This context is used by [StyleEditor.restoreCurrentStyle] after applying new styles.
func (se *StyleEditor) updateCurrentStyle(escapeSeq string) {
	if isResetSequence(escapeSeq) {
		se.currentStyle = ""
	} else {
		se.currentStyle = escapeSeq
	}
}

// processRegularCharacter handles regular (non-escape) characters.
func (se *StyleEditor) processRegularCharacter() {
	// Check if current plain position has a style to apply.
	style, ok := se.getStyleForCurrentPosition()
	if ok {
		// Process as styled character group.
		se.processStyledCharacterGroup(style)
	} else {
		// Process as unstyled characters.
		se.processUnstyled()
	}
}

// getStyleForCurrentPosition returns the style for the current plain text
// position if it exists and is valid. Returns the style and true if found, or
// zero value and false if no style exists or position is out of bounds.
//
// Examples:
//   - plainIdx=6, styleMap={6: boldStyle} -> return boldStyle, true
//   - plainIdx=3, styleMap={6: boldStyle} -> return emptyStyle, false
//   - plainIdx=15, len(plainRunes)=10 -> return emptyStyle, false (out of bounds)
func (se *StyleEditor) getStyleForCurrentPosition() (lipgloss.Style, bool) {
	if !se.isValidPlainPosition() {
		return lipgloss.NewStyle(), false
	}

	style, ok := se.styleMap[se.plainIdx]
	if !ok {
		return lipgloss.NewStyle(), false
	}

	return style, true
}

// isValidPlainPosition checks if the current plain text index is within bounds.
func (se *StyleEditor) isValidPlainPosition() bool {
	return se.plainIdx < len(se.plainRunes)
}

// processStyledCharacterGroup handles a group of consecutive characters with the same style.
func (se *StyleEditor) processStyledCharacterGroup(style lipgloss.Style) {
	// Find the extent of consecutive characters with the same style.
	groupStart, groupEnd := se.findStyleGroup(style)

	// Extract plain text for the group.
	groupText := string(se.plainRunes[groupStart:groupEnd])

	// Apply the style to the plain text group.
	styledText := style.Render(groupText)
	se.result.WriteString(styledText)

	// Restore any previous ANSI styling context.
	se.restoreCurrentStyle()

	// Skip past all processed characters in both plain and styled text.
	se.skipProcessedCharacters(groupEnd - groupStart)
}

// findStyleGroup finds consecutive characters with the same style starting from the current position.
func (se *StyleEditor) findStyleGroup(style lipgloss.Style) (int, int) {
	// Start with current position as group start.
	groupStart := se.plainIdx
	groupEnd := se.plainIdx + 1

	// Look ahead to find consecutive positions with identical style.
	// Stop when style changes or end of text reached.
	for groupEnd < len(se.plainRunes) {
		nextStyle, hasNext := se.styleMap[groupEnd]
		if !hasNext || !StylesEqual(style, nextStyle) {
			break
		}

		groupEnd++
	}

	return groupStart, groupEnd
}

// restoreCurrentStyle restores the current styling context if there was one.
func (se *StyleEditor) restoreCurrentStyle() {
	if se.currentStyle != "" {
		se.result.WriteString(se.currentStyle)
	}
}

// skipProcessedCharacters advances both indices past the processed characters.
func (se *StyleEditor) skipProcessedCharacters(charactersProcessed int) {
	// Advance plainIdx by the number of characters processed.
	se.plainIdx += charactersProcessed

	// Advance styledIdx by scanning through the styled text.
	// Count only visible characters until we've skipped the required amount.
	for i := 0; i < charactersProcessed && se.styledIdx < len(se.styledRunes); {
		if se.styledRunes[se.styledIdx] == ansi.ESC {
			// Skip over any ANSI escape sequences encountered.
			se.styledIdx = se.skipAnsiSequence(se.styledIdx)
		} else {
			se.styledIdx++
			i++
		}
	}
}

// skipAnsiSequence skips over an ANSI escape sequence starting at the given index.
// It returns the new index position after the [ansiTerminator].
func (se *StyleEditor) skipAnsiSequence(startIdx int) int {
	// Start at the given index (should be at ESC character).
	idx := startIdx

	// Scan forward until ansiTerminator is found.
	for idx < len(se.styledRunes) && se.styledRunes[idx] != ansiTerminator {
		idx++
	}
	if idx < len(se.styledRunes) {
		idx++ // Skip the [ansiTerminator].
	}

	return idx
}

// processUnstyled handles a regular character with no additional styling.
// The character is copied directly from styled text to result.
func (se *StyleEditor) processUnstyled() {
	se.result.WriteRune(se.styledRunes[se.styledIdx])

	se.plainIdx++
	se.styledIdx++
}

// isResetSequence checks if the given escape sequence is a SGR reset.
func isResetSequence(escapeSeq string) bool {
	return escapeSeq == ansi.ResetStyle || escapeSeq == ansi.SGR(ansi.ResetAttr)
}

// StylesEqual compares two [lipgloss.Style] objects for equality.
// This is a simple comparison that checks if the rendered output would be the same.
func StylesEqual(a, b lipgloss.Style) bool {
	// Compare by rendering a test character and checking if the output is identical.
	return a.Render("x") == b.Render("x")
}

// validateRanges validates style ranges and panics if any are invalid.
// Invalid ranges include those with Start > End or negative positions.
// Zero-length ranges (Start == End) are allowed and will be no-ops.
func validateRanges(ranges []StyleRange) {
	for i := range ranges {
		r := ranges[i]
		if r.Start < 0 {
			panic(fmt.Sprintf("invalid range at index %d: Start cannot be negative (got %d)", i, r.Start))
		}
		if r.End < 0 {
			panic(fmt.Sprintf("invalid range at index %d: End cannot be negative (got %d)", i, r.End))
		}
		if r.Start > r.End {
			panic(fmt.Sprintf(
				"invalid range at index %d: Start must be less than or equal to End (got Start=%d, End=%d)",
				i, r.Start, r.End,
			))
		}
	}
}
