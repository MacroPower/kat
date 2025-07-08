package statusbar

import (
	"fmt"
	"math"
	"strings"

	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/version"
)

const (
	helpText  = " ? Help "
	errorText = " ! Error "
)

type Style int

const (
	StyleNormal Style = iota
	StyleSuccess
	StyleError
)

// StatusBarRenderer handles status bar rendering for the pager.
type StatusBarRenderer struct {
	theme   *theme.Theme
	message string
	width   int
	style   Style
}

// NewStatusBarRenderer creates a new StatusBarRenderer.
func NewStatusBarRenderer(t *theme.Theme, width int, opts ...StatusBarOpt) *StatusBarRenderer {
	sb := &StatusBarRenderer{theme: t, width: width, style: StyleNormal}
	for _, opt := range opts {
		opt(sb)
	}

	return sb
}

type StatusBarOpt func(*StatusBarRenderer)

func WithMessage(message string, style Style) StatusBarOpt {
	return func(r *StatusBarRenderer) {
		r.style = style
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
		return r.theme.ErrorTitleStyle.Render(note)
	case StyleSuccess:
		return r.theme.StatusBarMessagePosStyle.Render(note)
	default:
		return r.theme.StatusBarPosStyle.Render(note)
	}
}

// renderHelpNote renders the help note component.
func (r *StatusBarRenderer) renderHelpNote() string {
	switch r.style {
	case StyleError:
		return r.theme.ErrorTitleStyle.Render(errorText)
	case StyleSuccess:
		return r.theme.StatusBarMessageHelpStyle.Render(helpText)
	default:
		return r.theme.StatusBarHelpStyle.Render(helpText)
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

	note = truncate.StringWithTail(" "+note+" ", uint(availableWidth), r.theme.Ellipsis) //nolint:gosec // Uses max.

	switch r.style {
	case StyleError:
		return r.theme.ErrorTitleStyle.Render(note)
	case StyleSuccess:
		return r.theme.StatusBarMessageStyle.Render(note)
	default:
		return r.theme.StatusBarStyle.Render(note)
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
		return r.theme.ErrorTitleStyle.Render(emptySpace)
	case StyleSuccess:
		return r.theme.StatusBarMessageStyle.Render(emptySpace)
	default:
		return r.theme.StatusBarStyle.Render(emptySpace)
	}
}

func (r *StatusBarRenderer) katLogoView() string {
	return r.theme.LogoStyle.Render(fmt.Sprintf(" kat %s ", version.GetVersion()))
}
