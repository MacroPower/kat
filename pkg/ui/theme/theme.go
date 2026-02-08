package theme

import (
	"errors"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"go.jacobcolvin.com/niceyaml/style"
	"golang.org/x/term"

	nytheme "go.jacobcolvin.com/niceyaml/style/theme"
)

const (
	// Ellipsis is the character used to indicate truncated text.
	Ellipsis = "…"

	defaultDarkTheme  = "github-dark"
	defaultLightTheme = "github"
)

const (
	// Overlay is a custom style key for overlay borders.
	Overlay style.Style = "overlay"
	// OverlayError is a custom style key for error overlay borders.
	OverlayError style.Style = "overlayError"
)

var (
	Default = New(getDefaultStyle())

	ErrInvalidName = errors.New("invalid theme name")
)

// Theme holds all visual styles for the application.
type Theme struct {
	Styles   style.Styles
	Ellipsis string
}

// Style returns the [lipgloss.Style] for the given semantic style key.
func (t *Theme) Style(key style.Style) lipgloss.Style {
	return *t.Styles.Style(key)
}

func New(themeName string) *Theme {
	ss := resolveStyles(themeName)

	textFg := ss.Style(style.Text).GetForeground()

	genericStyle := lipgloss.NewStyle().Foreground(textFg)

	return &Theme{
		Styles: ss.With(
			style.Set(Overlay, genericStyle.Border(lipgloss.RoundedBorder())),
			style.Set(OverlayError, genericStyle.
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ss.Style(style.TextError).GetForeground())),
		),
		Ellipsis: Ellipsis,
	}
}

// Register registers a custom theme by name with the given [style.Styles].
func Register(name string, ss style.Styles) error {
	if name == "" {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}

	// Default to dark mode for custom themes - most terminal users prefer dark.
	nytheme.Register(name, func() style.Styles { return ss }, style.Dark)

	return nil
}

func resolveStyles(themeName string) style.Styles {
	name := getStyle(themeName)

	ss, ok := nytheme.Styles(name)
	if ok {
		return ss
	}

	// Fallback to github-dark.
	ss, _ = nytheme.Styles(defaultDarkTheme)

	return ss
}

func getStyle(s string) string {
	switch s {
	case "dark":
		return defaultDarkTheme
	case "light":
		return defaultLightTheme
	case "auto", "":
		return getDefaultStyle()
	default:
		return s
	}
}

func getDefaultStyle() string {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return defaultDarkTheme // Fallback.
	}
	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		return defaultDarkTheme
	}

	return "github"
}
