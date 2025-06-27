package overlay

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/termenv"

	charmansi "github.com/charmbracelet/x/ansi"

	"github.com/MacroPower/kat/pkg/ui/themes"
)

const (
	defaultMinOverlayWidth = 16
)

type Overlay struct {
	ws    *whitespace
	theme *themes.Theme

	width, height int

	// Minimum width of the overlay.
	minWidth int
}

func New(theme *themes.Theme, opts ...OverlayOpt) *Overlay {
	ws := &whitespace{}
	o := &Overlay{
		ws:       ws,
		theme:    theme,
		minWidth: defaultMinOverlayWidth,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

type OverlayOpt func(*Overlay)

// WithMinWidth sets the minimum width of the overlay (in cells).
func WithMinWidth(minWidth int) OverlayOpt {
	return func(o *Overlay) {
		o.minWidth = minWidth
	}
}

// SetSize sets the size of the view on which the overlay is placed.
func (o *Overlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

// PlaceOverlay places fg on top of bg.
// From https://github.com/charmbracelet/lipgloss/pull/102
func (o *Overlay) Place(bg, fg string, widthFraction float64, style lipgloss.Style) string {
	overlayWidth := int(float64(o.width) * widthFraction)
	overlayWidth = clamp(overlayWidth, o.minWidth, o.width)

	fg = cellbuf.Wrap(fg, overlayWidth, " /-")
	fgLines, _ := getLines(fg)
	fgHeight := len(fgLines)
	maxHeight := o.height - 8
	if maxHeight < 1 {
		fgLines = []string{}
	} else if fgHeight > maxHeight {
		fgLines = fgLines[:maxHeight]
		maxTextWidth := uint(max(0, overlayWidth-4)) //nolint:gosec // G115: integer overflow conversion int -> uint.
		helperText := "output truncated; press <!> to view full output"
		helperText = truncate.StringWithTail(helperText, maxTextWidth, o.theme.Ellipsis)
		fgLines = append(fgLines, "", o.theme.SubtleStyle.Render(helperText))
	}

	fg = strings.Join(fgLines, "\n")

	fg = style.Width(overlayWidth).Render(fg)

	fgLines, fgWidth := getLines(fg)
	bgLines, bgWidth := getLines(bg)
	bgHeight := len(bgLines)
	fgHeight = len(fgLines)

	x := clamp(bgWidth-fgWidth, 0, bgWidth) / 2
	y := clamp(bgHeight-fgHeight, 0, bgHeight) / 2

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)

			continue
		}

		pos := 0
		if x > 0 {
			left := truncate.String(bgLine, uint(x))
			pos = ansi.PrintableRuneWidth(left)
			b.WriteString(left)
			if pos < x {
				b.WriteString(o.ws.render(x - pos))
				pos = x
			}
		}

		fgLine := fgLines[i-y]
		b.WriteString(fgLine)
		pos += ansi.PrintableRuneWidth(fgLine)

		right := charmansi.TruncateLeft(bgLine, pos, "")
		bgWidth = ansi.PrintableRuneWidth(bgLine)
		rightWidth := ansi.PrintableRuneWidth(right)
		if rightWidth <= bgWidth-pos {
			b.WriteString(o.ws.render(bgWidth - rightWidth - pos))
		}

		b.WriteString(right)
	}

	return b.String()
}

func clamp(v, lower, upper int) int {
	return min(max(v, lower), upper)
}

// Split a string into lines, additionally returning the size of the widest line.
func getLines(s string) ([]string, int) {
	lines := strings.Split(s, "\n")
	widest := 0
	for _, l := range lines {
		w := charmansi.StringWidth(l)
		if widest < w {
			widest = w
		}
	}

	return lines, widest
}

// whitespace is a whitespace renderer.
type whitespace struct {
	chars string
	style termenv.Style
}

// Render whitespaces.
func (w whitespace) render(width int) string {
	chars := " "
	if w.chars != "" {
		chars = w.chars
	}

	r := []rune(chars)
	j := 0
	b := strings.Builder{}

	// Cycle through runes and print them into the whitespace.
	for i := 0; i < width; {
		b.WriteRune(r[j])
		j++
		if j >= len(r) {
			j = 0
		}
		i += charmansi.StringWidth(string(r[j]))
	}

	// Fill any extra gaps white spaces. This might be necessary if any runes
	// are more than one cell wide, which could leave a one-rune gap.
	short := width - charmansi.StringWidth(b.String())
	if short > 0 {
		b.WriteString(strings.Repeat(" ", short))
	}

	return w.style.Styled(b.String())
}
