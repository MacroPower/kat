// Package overlay provides terminal UI overlay functionality for placing
// foreground content on top of background content, similar to modal dialogs
// or popup windows in terminal applications.
package overlay

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"

	charmansi "github.com/charmbracelet/x/ansi"

	"github.com/macropower/kat/pkg/ui/theme"
)

const (
	// Default minimum width for overlays in terminal cells.
	defaultMinWidth = 16
	// Default minimum padding reserved for overlay height calculations.
	// This ensures overlays don't consume the entire terminal height.
	defaultMinHeightPadding = 8

	wrapOnCharacters = " /-"
)

// Overlay manages the placement of foreground content over background content
// in terminal applications. It handles sizing, positioning, and text wrapping
// to create modal-like experiences.
type Overlay struct {
	theme *theme.Theme

	width, height int

	// Minimum width of the overlay.
	minWidth int

	// Minimum height padding reserved for overlays.
	minHeightPadding int
}

// New creates a new [Overlay] with the specified theme and optional configuration.
// The overlay is initialized with default settings and can be customized using
// the provided [OverlayOpt]s.
func New(t *theme.Theme, opts ...OverlayOpt) *Overlay {
	o := &Overlay{
		theme:            t,
		minWidth:         defaultMinWidth,
		minHeightPadding: defaultMinHeightPadding,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// OverlayOpt is a functional option for configuring an [Overlay].
type OverlayOpt func(*Overlay)

// WithMinWidth sets the minimum width of the overlay (in cells).
func WithMinWidth(minWidth int) OverlayOpt {
	return func(o *Overlay) {
		o.minWidth = minWidth
	}
}

// WithMinHeightPadding sets the minimum height padding reserved for overlays.
func WithMinHeightPadding(padding int) OverlayOpt {
	return func(o *Overlay) {
		o.minHeightPadding = padding
	}
}

// SetSize sets the size of the view on which the overlay is placed.
func (o *Overlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

// Place overlays the foreground content on top of the background content.
// The fg content is positioned in the center of the bg content, with the
// widthFraction determining how much of the available width should be used.
// The style is applied to the foreground content for borders, colors, etc.
//
// Content that exceeds the available height will be truncated with a helpful
// message indicating how to view the full output.
//
// From https://github.com/charmbracelet/lipgloss/pull/102
func (o *Overlay) Place(bg, fg string, widthFraction float64, style lipgloss.Style) string {
	overlayWidth := o.calculateOverlayDimensions(widthFraction)
	fgLines := o.prepareOverlayContent(fg, overlayWidth)

	// Apply styling to the final foreground content.
	styledFg := style.Width(overlayWidth).Render(strings.Join(fgLines, "\n"))

	return o.renderOverlay(bg, styledFg)
}

// renderOverlay combines the background and foreground content with proper positioning.
func (o *Overlay) renderOverlay(bg, fg string) string {
	fgLines, fgWidth := getLines(fg)
	bgLines, bgWidth := getLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	x, y := calculatePosition(bgWidth, bgHeight, fgWidth, fgHeight)

	// Pre-allocate builder with estimated capacity to reduce allocations.
	estimatedSize := len(bg) + len(fg) + (bgHeight * 10)
	var b strings.Builder
	b.Grow(estimatedSize)

	bgLineWidths := preCalculateLineWidths(bgLines)
	fgLineWidths := preCalculateLineWidths(fgLines)

	return o.buildCombinedOutput(&b, bgLines, fgLines, bgLineWidths, fgLineWidths, x, y)
}

// buildCombinedOutput constructs the final output by merging background and foreground content.
func (o *Overlay) buildCombinedOutput(b *strings.Builder, bgLines, fgLines []string, bgLineWidths, fgLineWidths []int, x, y int) string {
	fgHeight := len(fgLines)

	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}

		// Render background lines that don't overlap with the foreground.
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)

			continue
		}

		// Render lines where background and foreground overlap.
		o.renderOverlappingLine(b, bgLine, fgLines[i-y], bgLineWidths[i], fgLineWidths[i-y], x)
	}

	return b.String()
}

// renderOverlappingLine handles the complex logic of combining background and foreground on a single line.
func (o *Overlay) renderOverlappingLine(b *strings.Builder, bgLine, fgLine string, bgLineWidth, fgLineWidth, x int) {
	pos := 0

	// Render left background portion if x offset is positive.
	if x > 0 {
		pos = o.renderLeftBackground(b, bgLine, x)
	}

	// Render the foreground content.
	b.WriteString(fgLine)
	pos += fgLineWidth

	// Render right background portion if there's remaining content.
	if pos < bgLineWidth {
		o.renderRightBackground(b, bgLine, pos, bgLineWidth)
	}
}

// renderLeftBackground renders the left portion of the background line before the overlay.
func (o *Overlay) renderLeftBackground(b *strings.Builder, bgLine string, x int) int {
	left := truncate.String(bgLine, uint(x)) //nolint:gosec // G115: integer overflow conversion int -> uint.
	leftWidth := ansi.PrintableRuneWidth(left)
	b.WriteString(left)

	// Fill any gap with whitespace if truncated content is narrower than expected.
	if leftWidth < x {
		gap := x - leftWidth
		renderWhitespace(b, gap)

		return x
	}

	return leftWidth
}

// renderRightBackground renders the right portion of the background line after the overlay.
func (o *Overlay) renderRightBackground(b *strings.Builder, bgLine string, pos, bgLineWidth int) {
	right := charmansi.TruncateLeft(bgLine, pos, "")
	rightWidth := ansi.PrintableRuneWidth(right)

	// Add whitespace padding if needed to maintain proper alignment.
	paddingNeeded := bgLineWidth - rightWidth - pos
	if paddingNeeded > 0 {
		renderWhitespace(b, paddingNeeded)
	}

	b.WriteString(right)
}

// calculateOverlayDimensions determines the width and height for the overlay content.
func (o *Overlay) calculateOverlayDimensions(widthFraction float64) int {
	overlayWidth := int(float64(o.width) * widthFraction)

	return clamp(overlayWidth, o.minWidth, o.width)
}

// prepareOverlayContent wraps and truncates the content to fit within the overlay.
func (o *Overlay) prepareOverlayContent(content string, overlayWidth int) []string {
	// Wrap content to fit the overlay width.
	wrappedContent := cellbuf.Wrap(content, overlayWidth, wrapOnCharacters)
	contentLines := strings.Split(wrappedContent, "\n")

	maxContentHeight := o.height - o.minHeightPadding
	if maxContentHeight <= 0 {
		return []string{} // No space available for content.
	}

	if len(contentLines) <= maxContentHeight {
		return contentLines // Content fits without truncation.
	}

	// Truncate content and add helper text.
	truncatedLines := contentLines[:maxContentHeight]
	helperText := o.createTruncationMessage(overlayWidth)

	return append(truncatedLines, "", helperText)
}

// createTruncationMessage generates a styled message indicating content truncation.
func (o *Overlay) createTruncationMessage(overlayWidth int) string {
	maxTextWidth := uint(max(0, overlayWidth-4)) //nolint:gosec // G115: integer overflow conversion int -> uint.
	helperText := "output truncated; press <!> to view full output"
	truncatedText := truncate.StringWithTail(helperText, maxTextWidth, o.theme.Ellipsis)

	return o.theme.SubtleStyle.Render(truncatedText)
}

// calculatePosition determines the center position for the overlay within the background.
// Returns the x and y coordinates for the overlay's top-left corner.
func calculatePosition(bgWidth, bgHeight, fgWidth, fgHeight int) (int, int) {
	x := clamp(bgWidth-fgWidth, 0, bgWidth) / 2
	y := clamp(bgHeight-fgHeight, 0, bgHeight) / 2

	return x, y
}

// preCalculateLineWidths computes line widths in advance for performance.
func preCalculateLineWidths(lines []string) []int {
	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = ansi.PrintableRuneWidth(line)
	}

	return widths
}

// clamp constrains a value to be within the specified lower and upper bounds.
func clamp(v, lower, upper int) int {
	return min(max(v, lower), upper)
}

// getLines splits a string into lines and returns both the lines and the width
// of the widest line. This is used for calculating overlay positioning and sizing.
func getLines(s string) ([]string, int) {
	if s == "" {
		return []string{""}, 0
	}

	lines := strings.Split(s, "\n")
	widest := 0

	// Process all lines in a single pass for better cache locality.
	for _, line := range lines {
		if w := charmansi.StringWidth(line); w > widest {
			widest = w
		}
	}

	return lines, widest
}

// render generates a whitespace string of the specified width.
// It cycles through the configured characters and handles proper
// width calculation for multi-byte characters.
func renderWhitespace(b *strings.Builder, width int) {
	if width <= 0 {
		return
	}

	for range width {
		b.WriteByte(' ')
	}
}
