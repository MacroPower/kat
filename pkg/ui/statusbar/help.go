package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/MacroPower/kat/pkg/ui/common"
)

type KeyBindRenderer interface {
	Render(width int) string
}

var helpViewStyle = lipgloss.NewStyle().
	Foreground(statusBarNoteFg).
	Background(lipgloss.AdaptiveColor{Light: "#f2f2f2", Dark: "#1B1B1B"}).
	Render

// HelpRenderer handles help view rendering for the pager.
type HelpRenderer struct {
	keyBinds KeyBindRenderer
}

// NewHelpRenderer creates a new HelpViewRenderer.
func NewHelpRenderer(keyBinds KeyBindRenderer) *HelpRenderer {
	return &HelpRenderer{keyBinds: keyBinds}
}

// RenderHelpView renders the complete help view for the pager.
func (r *HelpRenderer) Render(width int) string {
	content := "\n" + r.keyBinds.Render(width)

	// Apply indentation.
	content = common.Indent(content, 1)

	// Apply styling.
	return helpViewStyle(content)
}

// CalculateHelpHeight calculates the height needed for the help view.
func (r *HelpRenderer) CalculateHelpHeight() int {
	helpContent := r.Render(0)

	return strings.Count(helpContent, "\n")
}
