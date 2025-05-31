package statusbar

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"

	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/version"
)

const (
	helpText  = " ? Help "
	errorText = " ! Error "
)

type StatusBarStyle int

const (
	StyleNormal StatusBarStyle = iota
	StyleSuccess
	StyleError
)

var (
	messageFg = lipgloss.AdaptiveColor{Light: "#89F0CB", Dark: "#89F0CB"}
	messageBg = lipgloss.AdaptiveColor{Light: "#1C8760", Dark: "#1C8760"}

	errorFg = styles.Gray
	errorBg = styles.Red

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
				Foreground(messageFg).
				Background(messageBg).
				Render

	statusBarMessageScrollPosStyle = lipgloss.NewStyle().
					Foreground(messageFg).
					Background(messageBg).
					Render

	statusBarMessageHelpStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#B6FFE4")).
					Background(styles.Green).
					Render

	statusBarErrorStyle = lipgloss.NewStyle().
				Foreground(errorFg).
				Background(errorBg).
				Render

	statusBarErrorScrollPosStyle = lipgloss.NewStyle().
					Foreground(errorFg).
					Background(errorBg).
					Render

	statusBarErrorHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFB6B6")).
				Background(styles.Gray).
				Render
)

// StatusBarRenderer handles status bar rendering for the pager.
type StatusBarRenderer struct {
	message string
	width   int
	style   StatusBarStyle
}

// NewStatusBarRenderer creates a new StatusBarRenderer.
func NewStatusBarRenderer(width int, opts ...StatusBarOpt) *StatusBarRenderer {
	sb := &StatusBarRenderer{width: width, style: StyleNormal}
	for _, opt := range opts {
		opt(sb)
	}

	return sb
}

type StatusBarOpt func(*StatusBarRenderer)

func WithMessage(message string) StatusBarOpt {
	return func(r *StatusBarRenderer) {
		r.style = StyleSuccess
		r.message = message
	}
}

func WithError(message string) StatusBarOpt {
	return func(r *StatusBarRenderer) {
		r.style = StyleError
		r.message = message
	}
}

func (r *StatusBarRenderer) getMessage(msg string) string {
	if r.message != "" {
		return r.message
	}

	return msg
}

// RenderWithScroll renders the complete status bar for the pager.
func (r *StatusBarRenderer) RenderWithScroll(msg string, scrollPercent float64) string {
	// Generate individual components.
	logo := r.katLogoView()
	scrollPercentText := r.renderScrollPercent(scrollPercent)
	helpNote := r.renderHelpNote()
	note := r.renderNote(msg, scrollPercentText)
	emptySpace := r.renderEmptySpace(logo, note, scrollPercentText, helpNote)

	return fmt.Sprintf("%s%s%s%s%s", logo, note, emptySpace, scrollPercentText, helpNote)
}

func (r *StatusBarRenderer) RenderWithNote(msg, progress string) string {
	// Generate individual components.
	logo := r.katLogoView()
	helpNote := r.renderHelpNote()
	progressNote := r.renderProgressNote(progress)
	note := r.renderNote(msg, progressNote)
	emptySpace := r.renderEmptySpace(logo, note, progressNote, helpNote)

	return fmt.Sprintf("%s%s%s%s%s", logo, note, emptySpace, progressNote, helpNote)
}

// renderScrollPercent renders the scroll percentage component.
func (r *StatusBarRenderer) renderScrollPercent(scrollPercent float64) string {
	percent := math.Max(0.0, math.Min(1.0, scrollPercent))
	scrollPercentStr := fmt.Sprintf("%3.f%%", percent*100.0)

	return r.renderProgressNote(scrollPercentStr)
}

func (r *StatusBarRenderer) renderProgressNote(note string) string {
	note = " " + note + " "

	switch r.style {
	case StyleError:
		return statusBarErrorScrollPosStyle(note)
	case StyleSuccess:
		return statusBarMessageScrollPosStyle(note)
	default:
		return statusBarScrollPosStyle(note)
	}
}

// renderHelpNote renders the help note component.
func (r *StatusBarRenderer) renderHelpNote() string {
	switch r.style {
	case StyleError:
		return statusBarErrorHelpStyle(errorText)
	case StyleSuccess:
		return statusBarMessageHelpStyle(helpText)
	default:
		return statusBarHelpStyle(helpText)
	}
}

// renderNote renders the main note/message component.
func (r *StatusBarRenderer) renderNote(msg, progress string) string {
	note := r.getMessage(msg)
	note = strings.ReplaceAll(note, "\n", " ") // Remove newlines for better rendering.
	note = strings.TrimSpace(note)             // Trim leading/trailing spaces.

	// Calculate available width for the note.
	logo := r.katLogoView()
	helpNote := r.renderHelpNote()

	availableWidth := max(0, r.width-
		ansi.PrintableRuneWidth(logo)-
		ansi.PrintableRuneWidth(progress)-
		ansi.PrintableRuneWidth(helpNote))

	note = truncate.StringWithTail(" "+note+" ", uint(availableWidth), styles.Ellipsis) //nolint:gosec // Uses max.

	switch r.style {
	case StyleError:
		return statusBarErrorStyle(note)
	case StyleSuccess:
		return statusBarMessageStyle(note)
	default:
		return statusBarNoteStyle(note)
	}
}

// renderEmptySpace calculates and renders the empty space between components.
func (r *StatusBarRenderer) renderEmptySpace(components ...string) string {
	padding := r.width
	for _, comp := range components {
		padding -= ansi.PrintableRuneWidth(comp)
	}
	padding = max(0, padding)

	emptySpace := strings.Repeat(" ", padding)

	switch r.style {
	case StyleError:
		return statusBarErrorStyle(emptySpace)
	case StyleSuccess:
		return statusBarMessageStyle(emptySpace)
	default:
		return statusBarNoteStyle(emptySpace)
	}
}

func (r *StatusBarRenderer) katLogoView() string {
	return styles.LogoStyle.Render(fmt.Sprintf(" kat %s ", version.GetVersion()))
}
