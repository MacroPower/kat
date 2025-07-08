package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/macropower/kat/pkg/ui/theme"
)

type KeyBindRenderer interface {
	Render(width int) string
}

// HelpRenderer handles help view rendering for the pager.
type HelpRenderer struct {
	theme    *theme.Theme
	keyBinds KeyBindRenderer
}

// NewHelpRenderer creates a new HelpViewRenderer.
func NewHelpRenderer(t *theme.Theme, keyBinds KeyBindRenderer) *HelpRenderer {
	return &HelpRenderer{theme: t, keyBinds: keyBinds}
}

// RenderHelpView renders the complete help view for the pager.
func (r *HelpRenderer) Render(width int) string {
	content := lipgloss.NewStyle().
		Padding(1).
		Render(r.keyBinds.Render(width))

	// Apply styling.
	return r.theme.HelpStyle.Render(content)
}

// CalculateHelpHeight calculates the height needed for the help view.
func (r *HelpRenderer) CalculateHelpHeight() int {
	helpContent := r.Render(0)

	return strings.Count(helpContent, "\n")
}
