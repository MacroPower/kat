package pager

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/keys"
)

var helpViewStyle = lipgloss.NewStyle().
	Foreground(statusBarNoteFg).
	Background(lipgloss.AdaptiveColor{Light: "#f2f2f2", Dark: "#1B1B1B"}).
	Render

// HelpRenderer handles help view rendering for the pager.
type HelpRenderer struct {
	keyBinds *config.KeyBinds
	width    int
}

// NewHelpRenderer creates a new HelpViewRenderer.
func NewHelpRenderer(width int, keyBinds *config.KeyBinds) *HelpRenderer {
	return &HelpRenderer{width: width, keyBinds: keyBinds}
}

// RenderHelpView renders the complete help view for the pager.
func (r *HelpRenderer) RenderHelpView() string {
	kb := r.keyBinds
	kbr := keys.KeyBindRenderer{}
	kbr.AddColumn(
		*kb.Common.Up,
		*kb.Common.Down,
		*kb.Pager.PageUp,
		*kb.Pager.PageDown,
		*kb.Pager.HalfPageUp,
		*kb.Pager.HalfPageDown,
	)
	kbr.AddColumn(
		*kb.Pager.Home,
		*kb.Pager.End,
		*kb.Pager.Copy,
		*kb.Common.Reload,
		*kb.Common.Escape,
		*kb.Common.Quit,
	)

	content := "\n" + kbr.Render(r.width)

	// Apply indentation.
	content = common.Indent(content, 1)

	// Apply styling.
	return helpViewStyle(content)
}

// CalculateHelpHeight calculates the height needed for the help view.
func (r *HelpRenderer) CalculateHelpHeight() int {
	helpContent := r.RenderHelpView()

	return strings.Count(helpContent, "\n")
}
