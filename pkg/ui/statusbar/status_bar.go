package statusbar

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/styles"
)

const helpText = " ? Help "

var (
	mintGreen = lipgloss.AdaptiveColor{Light: "#89F0CB", Dark: "#89F0CB"}
	darkGreen = lipgloss.AdaptiveColor{Light: "#1C8760", Dark: "#1C8760"}

	statusBarNoteFg = lipgloss.AdaptiveColor{Light: "#656565", Dark: "#7D7D7D"}
	statusBarBg     = lipgloss.AdaptiveColor{Light: "#E6E6E6", Dark: "#242424"}

	statusBarScrollPosStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#949494", Dark: "#5A5A5A"}).
				Background(statusBarBg).
				Render

	statusBarNoteStyle = lipgloss.NewStyle().
				Foreground(statusBarNoteFg).
				Background(statusBarBg).
				Render

	statusBarHelpStyle = lipgloss.NewStyle().
				Foreground(statusBarNoteFg).
				Background(lipgloss.AdaptiveColor{Light: "#DCDCDC", Dark: "#323232"}).
				Render

	statusBarMessageStyle = lipgloss.NewStyle().
				Foreground(mintGreen).
				Background(darkGreen).
				Render

	statusBarMessageScrollPosStyle = lipgloss.NewStyle().
					Foreground(mintGreen).
					Background(darkGreen).
					Render

	statusBarMessageHelpStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#B6FFE4")).
					Background(styles.Green).
					Render
)

// StatusBarRenderer handles status bar rendering for the pager.
type StatusBarRenderer struct {
	width int
}

// NewStatusBarRenderer creates a new StatusBarRenderer.
func NewStatusBarRenderer(width int) *StatusBarRenderer {
	return &StatusBarRenderer{width: width}
}

// RenderWithScroll renders the complete status bar for the pager.
func (r *StatusBarRenderer) RenderWithScroll(title, msg string, scrollPercent float64) string {
	showStatusMessage := msg != ""

	// Generate individual components.
	logo := r.renderLogo()
	scrollPercentText := r.renderScrollPercent(scrollPercent, showStatusMessage)
	helpNote := r.renderHelpNote(showStatusMessage)
	note := r.renderNote(title, msg, scrollPercent)
	emptySpace := r.renderEmptySpace(logo, note, scrollPercentText, helpNote, showStatusMessage)

	return fmt.Sprintf("%s%s%s%s%s", logo, note, emptySpace, scrollPercentText, helpNote)
}

// renderLogo renders the logo component.
func (r *StatusBarRenderer) renderLogo() string {
	return common.KatLogoView()
}

// renderScrollPercent renders the scroll percentage component.
func (r *StatusBarRenderer) renderScrollPercent(scrollPercent float64, showStatusMessage bool) string {
	percent := math.Max(0.0, math.Min(1.0, scrollPercent))
	scrollPercentStr := fmt.Sprintf(" %3.f%% ", percent*100.0)

	if showStatusMessage {
		return statusBarMessageScrollPosStyle(scrollPercentStr)
	}

	return statusBarScrollPosStyle(scrollPercentStr)
}

// renderHelpNote renders the help note component.
func (r *StatusBarRenderer) renderHelpNote(showStatusMessage bool) string {
	if showStatusMessage {
		return statusBarMessageHelpStyle(helpText)
	}

	return statusBarHelpStyle(helpText)
}

// renderNote renders the main note/message component.
func (r *StatusBarRenderer) renderNote(title, msg string, scrollPercent float64) string {
	showStatusMessage := msg != ""

	var note string
	if msg != "" {
		note = msg
	} else {
		note = title
	}

	// Calculate available width for the note.
	logo := r.renderLogo()
	scrollPercentText := r.renderScrollPercent(scrollPercent, showStatusMessage)
	helpNote := r.renderHelpNote(showStatusMessage)

	availableWidth := max(0, r.width-
		ansi.PrintableRuneWidth(logo)-
		ansi.PrintableRuneWidth(scrollPercentText)-
		ansi.PrintableRuneWidth(helpNote))

	note = truncate.StringWithTail(" "+note+" ", uint(availableWidth), styles.Ellipsis) //nolint:gosec // Uses max.

	if showStatusMessage {
		return statusBarMessageStyle(note)
	}

	return statusBarNoteStyle(note)
}

// renderEmptySpace calculates and renders the empty space between components.
func (r *StatusBarRenderer) renderEmptySpace(logo, note, scrollPercent, helpNote string, showStatusMessage bool) string {
	padding := max(0, r.width-
		ansi.PrintableRuneWidth(logo)-
		ansi.PrintableRuneWidth(note)-
		ansi.PrintableRuneWidth(scrollPercent)-
		ansi.PrintableRuneWidth(helpNote))

	emptySpace := strings.Repeat(" ", padding)

	if showStatusMessage {
		return statusBarMessageStyle(emptySpace)
	}

	return statusBarNoteStyle(emptySpace)
}
