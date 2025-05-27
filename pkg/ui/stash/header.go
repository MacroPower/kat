package stash

import (
	"strings"

	"github.com/muesli/reflow/ansi"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/styles"
)

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
		content = styles.TruncateWithEllipsis(content, maxWidth)
	}

	return content
}
