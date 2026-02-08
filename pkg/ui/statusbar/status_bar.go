package statusbar

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"go.jacobcolvin.com/niceyaml/style"

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

// styleSet holds pre-computed styles for the current status bar state.
type styleSet struct {
	note, pos, help lipgloss.Style
}

// StatusBarRenderer handles status bar rendering. Create once with
// [NewStatusBarRenderer] and reuse; call [StatusBarRenderer.SetWidth] on
// resize and apply options before each render.
type StatusBarRenderer struct {
	theme   *theme.Theme
	logo    string
	message string
	width   int
	style   Style
}

// currentStyles returns the style set for the current [Style].
func (r *StatusBarRenderer) currentStyles() styleSet {
	switch r.style {
	case StyleError:
		return styleSet{
			note: r.theme.Style(style.TitleError),
			pos:  r.theme.Style(style.TitleError),
			help: r.theme.Style(style.TitleError),
		}

	case StyleSuccess:
		return styleSet{
			note: r.theme.Style(style.Title),
			pos:  r.theme.Style(style.Title),
			help: r.theme.Style(style.Title),
		}

	default:
		return styleSet{
			note: r.theme.Style(style.TitleSubtle),
			pos:  r.theme.Style(style.TitleAccent),
			help: r.theme.Style(style.TitleAccent),
		}
	}
}

// NewStatusBarRenderer creates a new [StatusBarRenderer]. The logo string is
// pre-rendered once and reused across frames.
func NewStatusBarRenderer(t *theme.Theme, width int, opts ...StatusBarOpt) *StatusBarRenderer {
	sb := &StatusBarRenderer{
		theme: t,
		width: width,
		style: StyleNormal,
		logo:  t.Style(style.Title).Render(fmt.Sprintf(" kat %s ", version.GetVersion())),
	}
	for _, opt := range opts {
		opt(sb)
	}

	return sb
}

// StatusBarOpt configures a [StatusBarRenderer] before rendering.
type StatusBarOpt func(*StatusBarRenderer)

// WithMessage sets a status message and style on the renderer.
func WithMessage(message string, s Style) StatusBarOpt {
	return func(r *StatusBarRenderer) {
		r.style = s
		r.message = message
	}
}

// SetWidth updates the renderer width (typically called on resize).
func (r *StatusBarRenderer) SetWidth(w int) {
	r.width = w
}

// Apply applies options to the renderer, resetting message state first.
// Use this to update the renderer before each render call.
func (r *StatusBarRenderer) Apply(opts ...StatusBarOpt) {
	r.message = ""
	r.style = StyleNormal

	for _, opt := range opts {
		opt(r)
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

	return r.currentStyles().pos.Render(note)
}

// renderHelpNote renders the help note component.
func (r *StatusBarRenderer) renderHelpNote() string {
	ss := r.currentStyles()

	text := helpText
	if r.style == StyleError {
		text = errorText
	}

	return ss.help.Render(text)
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
		ansi.StringWidth(logo)-
		ansi.StringWidth(progress)-
		ansi.StringWidth(helpNote))

	note = ansi.Truncate(" "+note+" ", availableWidth, r.theme.Ellipsis)

	return r.currentStyles().note.Render(note)
}

// renderEmptySpace calculates and renders the empty space between components.
func (r *StatusBarRenderer) renderEmptySpace(components ...string) string {
	padding := r.width
	for _, comp := range components {
		padding -= ansi.StringWidth(comp)
	}

	padding = max(0, padding)

	emptySpace := strings.Repeat(" ", padding)

	return r.currentStyles().note.Render(emptySpace)
}

func (r *StatusBarRenderer) katLogoView() string {
	return r.logo
}
