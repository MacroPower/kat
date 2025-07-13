package yamls

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/lipgloss"

	"github.com/macropower/kat/pkg/ui/ansis"
)

// DiffType represents the type of diff change.
type DiffType int

const (
	DiffInserted DiffType = iota
	DiffDeleted
	DiffEdited
)

// DiffPosition represents a diff change position within the content.
type DiffPosition struct {
	Line  int      // 0-based line number.
	Start int      // 0-based character position within the line.
	End   int      // 0-based character position within the line (exclusive).
	Type  DiffType // Type of change (added, removed, or changed).
}

// DiffHighlighter handles diff-specific highlighting via [*ansis.StyleEditor].
type DiffHighlighter struct {
	insertedStyle lipgloss.Style
	deletedStyle  lipgloss.Style // For future use.
	editedStyle   lipgloss.Style
}

// NewDiffHighlighter creates a new [DiffHighlighter].
func NewDiffHighlighter(insertedStyle, deletedStyle, editedStyle lipgloss.Style) *DiffHighlighter {
	return &DiffHighlighter{
		insertedStyle: insertedStyle,
		deletedStyle:  deletedStyle,
		editedStyle:   editedStyle,
	}
}

// ApplyDiffHighlights applies diff highlighting to content that already has chroma styling.
// It converts [DiffPosition] slices to [ansis.StyleRange] slices and delegates to the [*ansis.StyleEditor].
func (dh *DiffHighlighter) ApplyDiffHighlights(text string, diffs []DiffPosition) string {
	if len(diffs) == 0 {
		return text
	}

	lineRanges := dh.convertDiffsToStyleRanges(diffs)

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

// convertDiffsToStyleRanges converts [DiffPosition] slices to [ansis.StyleRange] slices organized by line.
func (dh *DiffHighlighter) convertDiffsToStyleRanges(diffs []DiffPosition) map[int][]ansis.StyleRange {
	lineRanges := map[int][]ansis.StyleRange{}

	for _, diff := range diffs {
		var style lipgloss.Style
		switch diff.Type {
		case DiffInserted:
			style = dh.insertedStyle
		case DiffDeleted:
			style = dh.deletedStyle
		case DiffEdited:
			style = dh.editedStyle
		}

		styleRange := ansis.StyleRange{
			Start:    diff.Start,
			End:      diff.End,
			Style:    style,
			Priority: 3, // Higher priority than search highlights to ensure diffs show on top.
		}

		lineRanges[diff.Line] = append(lineRanges[diff.Line], styleRange)
	}

	return lineRanges
}

type Differ struct {
	originalContent string
	diffs           []DiffPosition
}

// NewDiffer creates a new [Differ].
func NewDiffer() *Differ {
	return &Differ{}
}

// SetInitialContent sets the initial content for diff comparison.
func (dh *Differ) SetInitialContent(content string) {
	if dh.originalContent == "" {
		dh.originalContent = content
	}
}

// SetOriginalContent sets the original content for diff comparison.
func (dh *Differ) SetOriginalContent(content string) {
	if dh.originalContent != content {
		dh.originalContent = content
		dh.diffs = nil
	}
}

// FindAndCacheDiffs finds differences between the current content and the
// original content using go-udiff.
func (dh *Differ) FindDiffs(currentContent string) []DiffPosition {
	// Use go-udiff to compute differences.
	edits := udiff.Strings(dh.originalContent, currentContent)

	diffs := []DiffPosition{}
	for _, edit := range edits {
		if edit.New == "" {
			// Skip deletions since we only render current content.
			continue
		}

		// Convert edit to diff positions and append.
		diffs = append(diffs, convertEditToDiffPositions(edit, currentContent)...)
	}

	return diffs
}

// FindAndCacheDiffs calls [Differ.FindDiffs], stores the diffs internally and
// re-returns them if the content hasn't changed.
func (dh *Differ) FindAndCacheDiffs(currentContent string) []DiffPosition {
	// If the current content is the same as the original, return existing diffs.
	if dh.originalContent == currentContent {
		return dh.diffs
	}

	// Otherwise, find diffs and store the results.
	diffs := dh.FindDiffs(currentContent)
	dh.diffs = diffs

	// Update the original content to the current content, so that subsequent calls
	// return the same diffs if the content hasn't changed.
	dh.originalContent = currentContent

	return dh.diffs
}

// GetDiffs returns the current diff positions.
func (dh *Differ) GetDiffs() []DiffPosition {
	return dh.diffs
}

// ClearDiffs clears the current diff positions.
func (dh *Differ) ClearDiffs() {
	dh.diffs = nil
}

func (dh *Differ) Unload() {
	dh.ClearDiffs()

	dh.originalContent = ""
}

// convertEditToDiffPosition converts a [udiff.Edit] to [DiffPosition]s.
func convertEditToDiffPositions(edit udiff.Edit, currentContent string) []DiffPosition {
	// Find the line and position where this edit starts in the new content.
	currentLines := strings.Split(currentContent, "\n")

	// Calculate line and column for the edit start position.
	lineNum, startCol := offsetToLineCol(currentContent, edit.Start)
	if lineNum == -1 {
		return nil
	}

	// Check if this is a multi-line edit.
	newLines := strings.Split(edit.New, "\n")
	isMultiLine := len(newLines) > 1

	// Determine diff type.
	diffType := DiffInserted
	if edit.End > edit.Start {
		diffType = DiffEdited
	}

	if !isMultiLine && lineNum < len(currentLines) {
		// Single line edit - highlight only the changed portion.
		line := currentLines[lineNum]
		lineRunes := []rune(line)

		// Calculate the end position within the line.
		endCol := startCol + len([]rune(edit.New))

		// Ensure we don't go beyond the line length.
		endCol = min(endCol, len(lineRunes))

		return []DiffPosition{{
			Line:  lineNum,
			Start: startCol,
			End:   endCol,
			Type:  diffType,
		}}
	}

	// Multi-line edit, highlight full lines.
	diffs := []DiffPosition{}
	for i := range newLines {
		currentLineNum := lineNum + i
		if currentLineNum >= len(currentLines) {
			break
		}

		// For the first line of a multi-line edit, start from the column position
		// For subsequent lines, highlight the entire line.
		start := 0
		if i == 0 {
			start = startCol
		}

		lineType := diffType
		if i > 0 && diffType == DiffEdited {
			// Additional lines in a change are additions.
			lineType = DiffInserted
		}

		diffs = append(diffs, DiffPosition{
			Line:  currentLineNum,
			Start: start,
			End:   len([]rune(currentLines[currentLineNum])),
			Type:  lineType,
		})
	}

	return diffs
}

// offsetToLineCol converts a byte offset to line number and column position.
func offsetToLineCol(content string, offset int) (int, int) {
	if offset > len(content) {
		return -1, -1
	}

	var line, col, currentPos int

	for i, ch := range content {
		if i == offset {
			// Convert byte position to rune position for the column.
			col = len([]rune(content[currentPos:i]))
			return line, col
		}

		if ch == '\n' {
			line++
			currentPos = i + 1
		}
	}

	// If we've reached the end of content.
	if offset == len(content) {
		col = len([]rune(content[currentPos:]))
		return line, col
	}

	return -1, -1
}
