package pager

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	runewidth "github.com/mattn/go-runewidth"

	"github.com/MacroPower/kat/pkg/ui/common"
)

var helpViewStyle = lipgloss.NewStyle().
	Foreground(statusBarNoteFg).
	Background(lipgloss.AdaptiveColor{Light: "#f2f2f2", Dark: "#1B1B1B"}).
	Render

// HelpRenderer handles help view rendering for the pager.
type HelpRenderer struct {
	width int
}

// NewHelpRenderer creates a new HelpViewRenderer.
func NewHelpRenderer(width int) *HelpRenderer {
	return &HelpRenderer{width: width}
}

// RenderHelpView renders the complete help view for the pager.
func (r *HelpRenderer) RenderHelpView() string {
	// Define help commands.
	col1 := r.getHelpCommands()

	// Build the help content.
	content := r.buildHelpContent(col1)

	// Apply indentation.
	content = common.Indent(content, 2)

	// Fill up empty cells with spaces for background coloring.
	content = r.fillEmptySpaces(content)

	// Apply styling.
	return helpViewStyle(content)
}

// getHelpCommands returns the list of help commands.
func (r *HelpRenderer) getHelpCommands() []string {
	return []string{
		"g/home  go to top",
		"G/end   go to bottom",
		"c       copy contents",
		"r       reload this document",
		"esc     back to files",
		"q       quit",
	}
}

// buildHelpContent builds the formatted help content.
func (r *HelpRenderer) buildHelpContent(col1 []string) string {
	var s string

	s += "\n"
	s += "k/↑      up                  " + col1[0] + "\n"
	s += "j/↓      down                " + col1[1] + "\n"
	s += "b/pgup   page up             " + col1[2] + "\n"
	s += "f/pgdn   page down           " + col1[3] + "\n"
	s += "u        ½ page up           " + col1[4] + "\n"
	s += "d        ½ page down         "

	if len(col1) > 5 {
		s += col1[5]
	}

	return s
}

// fillEmptySpaces fills up empty cells with spaces for background coloring.
func (r *HelpRenderer) fillEmptySpaces(content string) string {
	if r.width <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i := range lines {
		l := runewidth.StringWidth(lines[i])
		n := max(r.width-l, 0)
		lines[i] += strings.Repeat(" ", n)
	}

	return strings.Join(lines, "\n")
}

// CalculateHelpHeight calculates the height needed for the help view.
func (r *HelpRenderer) CalculateHelpHeight() int {
	helpContent := r.RenderHelpView()

	return strings.Count(helpContent, "\n")
}
