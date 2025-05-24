package stash

import (
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// ViewBuilder helps build complex views in a composable way.
type ViewBuilder struct {
	sections []string
	styles   []lipgloss.Style
}

// NewViewBuilder creates a new view builder.
func NewViewBuilder() *ViewBuilder {
	return &ViewBuilder{
		sections: make([]string, 0),
		styles:   make([]lipgloss.Style, 0),
	}
}

// AddSection adds a section to the view.
func (vb *ViewBuilder) AddSection(content string, style ...lipgloss.Style) *ViewBuilder {
	vb.sections = append(vb.sections, content)
	if len(style) > 0 {
		vb.styles = append(vb.styles, style[0])
	} else {
		vb.styles = append(vb.styles, lipgloss.Style{})
	}

	return vb
}

// Build constructs the final view string.
func (vb *ViewBuilder) Build() string {
	if len(vb.sections) == 0 {
		return ""
	}

	var result strings.Builder
	for i, section := range vb.sections {
		if i < len(vb.styles) && !vb.styles[i].GetUnderline() {
			section = vb.styles[i].Render(section)
		}
		result.WriteString(section)
		if i < len(vb.sections)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// HeaderRenderer handles rendering of header sections.
type HeaderRenderer struct {
	width  int
	height int
}

// NewHeaderRenderer creates a new header renderer.
func NewHeaderRenderer(width, height int) *HeaderRenderer {
	return &HeaderRenderer{width: width, height: height}
}

// RenderLogo renders the kat logo.
func (hr *HeaderRenderer) RenderLogo() string {
	return common.KatLogoView()
}

// RenderLogoOrFilter renders either the logo or filter input based on state.
func (hr *HeaderRenderer) RenderLogoOrFilter(filterState FilterState, filterInput, statusMsg string) string {
	var result strings.Builder
	result.WriteString(" ")

	if filterState == Filtering {
		if statusMsg != "" {
			result.WriteString(statusMsg)
		} else {
			result.WriteString(filterInput)
		}
	} else {
		result.WriteString(hr.RenderLogo())
		if statusMsg != "" {
			result.WriteString("  " + statusMsg)
		}
	}

	// Truncate if too wide.
	content := result.String()
	maxWidth := hr.width - 1
	if maxWidth > 0 && ansi.PrintableRuneWidth(content) > maxWidth {
		content = truncateWithEllipsis(content, maxWidth)
	}

	return content
}

// PaginationRenderer handles pagination display.
type PaginationRenderer struct {
	width int
}

// NewPaginationRenderer creates a new pagination renderer.
func NewPaginationRenderer(width int) *PaginationRenderer {
	return &PaginationRenderer{width: width}
}

// RenderPagination renders pagination controls.
func (pr *PaginationRenderer) RenderPagination(paginatorModel *paginator.Model, totalPages int) string {
	if totalPages <= 1 {
		return ""
	}

	pagination := paginatorModel.View()

	// If the dot pagination is wider than available space, use arabic numerals.
	availableWidth := pr.width - stashViewHorizontalPadding
	if ansi.PrintableRuneWidth(pagination) > availableWidth {
		// Create a copy to avoid mutating the original.
		p := *paginatorModel
		p.Type = paginator.Arabic
		pagination = p.View()
	}

	return styles.PaginationStyle.Render(pagination)
}

// DocumentListRenderer handles rendering of document lists.
type DocumentListRenderer struct {
	width  int
	height int
}

// NewDocumentListRenderer creates a new document list renderer.
func NewDocumentListRenderer(width, height int) *DocumentListRenderer {
	return &DocumentListRenderer{width: width, height: height}
}

// RenderDocumentList renders a list of documents with pagination and empty states.
func (dlr *DocumentListRenderer) RenderDocumentList(docs []*yamldoc.YAMLDocument, m StashModel) string {
	var b strings.Builder

	// Handle empty states.
	if len(docs) == 0 {
		f := func(s string) {
			b.WriteString("  " + styles.GrayFg(s))
		}

		switch m.currentSection().key {
		case documentsSection:
			if m.loadingDone() {
				f("No files found.")
			} else {
				f("Looking for local files...")
			}
		case filterSection:
			return ""
		}
	}

	// Render documents with pagination.
	if len(docs) > 0 {
		start, end := m.paginator().GetSliceBounds(len(docs))
		pageItems := docs[start:end]

		for i, md := range pageItems {
			stashItemView(&b, m, i, md)
			if i != len(pageItems)-1 {
				b.WriteString("\n\n")
			}
		}
	}

	// Fill remaining space on page.
	itemsOnPage := m.paginator().ItemsOnPage(len(docs))
	if itemsOnPage < m.paginator().PerPage {
		n := (m.paginator().PerPage - itemsOnPage) * stashViewItemHeight
		if len(docs) == 0 {
			n -= stashViewItemHeight - 1
		}
		for range n {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// LayoutCalculator helps calculate layout dimensions.
type LayoutCalculator struct {
	totalHeight int
	totalWidth  int
}

// NewLayoutCalculator creates a new layout calculator.
func NewLayoutCalculator(width, height int) *LayoutCalculator {
	return &LayoutCalculator{
		totalWidth:  width,
		totalHeight: height,
	}
}

// CalculateAvailableHeight calculates available height for content.
func (lc *LayoutCalculator) CalculateAvailableHeight(topPadding, bottomPadding, helpHeight, contentHeight int) int {
	used := topPadding + bottomPadding + helpHeight + contentHeight
	available := lc.totalHeight - used

	return max(0, available)
}

// Utility functions for view rendering.

// truncateWithEllipsis truncates a string with ellipsis if it exceeds maxWidth.
func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.PrintableRuneWidth(s) <= maxWidth {
		return s
	}

	// Reserve space for ellipsis.
	if maxWidth <= len(styles.Ellipsis) {
		return styles.Ellipsis[:maxWidth]
	}

	// Simple truncation - could be improved with proper text handling.
	availableWidth := maxWidth - len(styles.Ellipsis)
	truncated := ""
	currentWidth := 0

	for _, r := range s {
		runeWidth := ansi.PrintableRuneWidth(string(r))
		if currentWidth+runeWidth > availableWidth {
			break
		}
		truncated += string(r)
		currentWidth += runeWidth
	}

	return truncated + styles.Ellipsis
}

// fillVerticalSpace creates newlines to fill vertical space.
func fillVerticalSpace(lines int) string {
	if lines <= 0 {
		return ""
	}

	return strings.Repeat("\n", lines)
}

// padHorizontal adds horizontal padding to content.
func padHorizontal(content string, leftPad, rightPad int) string {
	if leftPad <= 0 && rightPad <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	var result strings.Builder

	leftPadding := strings.Repeat(" ", leftPad)
	rightPadding := strings.Repeat(" ", rightPad)

	for i, line := range lines {
		result.WriteString(leftPadding + line + rightPadding)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
