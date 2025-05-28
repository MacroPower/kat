package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ViewBuilder helps build complex views in a composable way.
// It combines the functionality from both pager and stash view builders.
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

// AddSection adds a section to the view with optional styling.
func (vb *ViewBuilder) AddSection(content string, style ...lipgloss.Style) *ViewBuilder {
	if content == "" {
		return vb
	}

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

// FillVerticalSpace creates newlines to fill vertical space.
func FillVerticalSpace(lines int) string {
	if lines <= 0 {
		return ""
	}

	return strings.Repeat("\n", lines)
}
