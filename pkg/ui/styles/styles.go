package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
)

// Icons.
const (
	Ellipsis = "…"
)

// Colors.
var (
	NormalDim      = lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}
	Gray           = lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"}
	MidGray        = lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#4A4A4A"}
	DarkGray       = lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}
	BrightGray     = lipgloss.AdaptiveColor{Light: "#847A85", Dark: "#979797"}
	DimBrightGray  = lipgloss.AdaptiveColor{Light: "#C2B8C2", Dark: "#4D4D4D"}
	Cream          = lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}
	YellowGreen    = lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"}
	Fuchsia        = lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}
	DimFuchsia     = lipgloss.AdaptiveColor{Light: "#F1A8FF", Dark: "#99519E"}
	DullFuchsia    = lipgloss.AdaptiveColor{Dark: "#AD58B4", Light: "#F793FF"}
	DimDullFuchsia = lipgloss.AdaptiveColor{Light: "#F6C9FF", Dark: "#7B4380"}
	Green          = lipgloss.Color("#04B575")
	Red            = lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}
	SemiDimGreen   = lipgloss.AdaptiveColor{Light: "#35D79C", Dark: "#036B46"}
	DimGreen       = lipgloss.AdaptiveColor{Light: "#72D2B0", Dark: "#0B5137"}

	// Ulimately, we'll transition to named styles.
	DimNormalFg      = lipgloss.NewStyle().Foreground(NormalDim).Render
	BrightGrayFg     = lipgloss.NewStyle().Foreground(BrightGray).Render
	DimBrightGrayFg  = lipgloss.NewStyle().Foreground(DimBrightGray).Render
	GrayFg           = lipgloss.NewStyle().Foreground(Gray).Render
	MidGrayFg        = lipgloss.NewStyle().Foreground(MidGray).Render
	DarkGrayFg       = lipgloss.NewStyle().Foreground(DarkGray)
	GreenFg          = lipgloss.NewStyle().Foreground(Green).Render
	SemiDimGreenFg   = lipgloss.NewStyle().Foreground(SemiDimGreen).Render
	DimGreenFg       = lipgloss.NewStyle().Foreground(DimGreen).Render
	FuchsiaFg        = lipgloss.NewStyle().Foreground(Fuchsia).Render
	DimFuchsiaFg     = lipgloss.NewStyle().Foreground(DimFuchsia).Render
	DullFuchsiaFg    = lipgloss.NewStyle().Foreground(DullFuchsia).Render
	DimDullFuchsiaFg = lipgloss.NewStyle().Foreground(DimDullFuchsia).Render
	RedFg            = lipgloss.NewStyle().Foreground(Red).Render
	TabStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"})
	SelectedTabStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#979797"})
	ErrorTitleStyle  = lipgloss.NewStyle().Foreground(Cream).Background(Red).Padding(0, 1)
	SubtleStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"})
	PaginationStyle  = SubtleStyle.PaddingLeft(2).PaddingBottom(1)

	LogoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ECFD65")).
			Background(Fuchsia).
			Bold(true)
)

// TruncateWithEllipsis truncates a string with ellipsis if it exceeds maxWidth.
func TruncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		if s == "" {
			return ""
		}

		return Ellipsis
	}
	if ansi.PrintableRuneWidth(s) <= maxWidth {
		return s
	}

	lenEllipsis := ansi.PrintableRuneWidth(Ellipsis)

	// Reserve space for ellipsis.
	if maxWidth <= lenEllipsis {
		return Ellipsis[:maxWidth]
	}

	// Simple truncation - could be improved with proper text handling.
	availableWidth := maxWidth - lenEllipsis
	truncated := ""
	currentWidth := 0

	for _, r := range s {
		runeWidth := ansi.PrintableRuneWidth(string(r))
		if currentWidth+runeWidth > availableWidth {
			break
		}
		truncated += string(r)
		currentWidth += runeWidth
	}

	return truncated + Ellipsis
}
