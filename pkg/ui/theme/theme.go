package theme

import (
	"errors"
	"fmt"
	"image/color"
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

var (
	Default = New(getDefaultStyle())

	ErrInvalidName = errors.New("invalid theme name")
)

// Theme holds all visual styles for the application.
type Theme struct {
	StatusBar StatusBarStyles
	Error     ErrorStyles

	CursorStyle         lipgloss.Style
	ResultTitleStyle    lipgloss.Style
	FilterStyle         lipgloss.Style
	GenericOverlayStyle lipgloss.Style
	GenericTextStyle    lipgloss.Style
	HelpStyle           lipgloss.Style
	LineNumberStyle     lipgloss.Style
	LogoStyle           lipgloss.Style
	PaginationStyle     lipgloss.Style
	SelectedStyle       lipgloss.Style
	SelectedSubtleStyle lipgloss.Style
	SubtleStyle         lipgloss.Style
	InsertedStyle       lipgloss.Style
	DeletedStyle        lipgloss.Style

	// BackgroundColor is the theme's base background color, derived from the
	// text background of the active syntax theme.
	BackgroundColor color.Color

	NiceyamlStyles style.Styles
	Ellipsis       string
}

// StatusBarStyles groups styles used by the status bar.
type StatusBarStyles struct {
	Style            lipgloss.Style
	PosStyle         lipgloss.Style
	HelpStyle        lipgloss.Style
	MessageStyle     lipgloss.Style
	MessagePosStyle  lipgloss.Style
	MessageHelpStyle lipgloss.Style
}

// ErrorStyles groups styles used by error and overlay displays.
type ErrorStyles struct {
	OverlayStyle lipgloss.Style
	TitleStyle   lipgloss.Style
	TextStyle    lipgloss.Style
}

func New(themeName string) *Theme {
	ss := resolveStyles(themeName)

	// Extract colors from niceyaml styles.
	textFg := getStyleFg(ss, style.Text)
	textBg := getStyleBg(ss, style.Text)
	nameTagFg := getStyleFg(ss, style.NameTag)
	commentFg := getStyleFg(ss, style.Comment)
	deletedFg := getStyleFg(ss, style.GenericDeleted)
	insertedFg := getStyleFg(ss, style.GenericInserted)

	var (
		genericStyle = lipgloss.NewStyle().
				Foreground(textFg)

		logoStyle = lipgloss.NewStyle().
				Foreground(textBg).
				Background(nameTagFg).
				Bold(true)

		selectedStyle = lipgloss.NewStyle().
				Foreground(nameTagFg)

		selectedSubtleStyle = lipgloss.NewStyle().
					Foreground(brighten(nameTagFg, textBg, 0.3))

		filterStyle = selectedStyle

		cursorStyle = selectedSubtleStyle

		helpStyle = lipgloss.NewStyle().
				Foreground(brighten(textFg, textBg, 0.2)).
				Background(brighten(textBg, textFg, 0.2))

		statusBarStyle = lipgloss.NewStyle().
				Foreground(textFg).
				Background(brighten(textBg, textFg, 0.1))

		statusBarPosStyle = lipgloss.NewStyle().
					Foreground(textFg).
					Background(brighten(textBg, textFg, 0.15))

		statusBarHelpStyle = helpStyle

		statusBarMessageStyle = lipgloss.NewStyle().
					Foreground(textBg).
					Background(brighten(nameTagFg, textBg, 0.15))

		statusBarMessagePosStyle = lipgloss.NewStyle().
						Foreground(textBg).
						Background(brighten(nameTagFg, textBg, 0.1))

		statusBarMessageHelpStyle = genericStyle.
						Foreground(textBg).
						Background(nameTagFg)

		errorTitleStyle = genericStyle.
				Foreground(textBg).
				Background(deletedFg).
				Bold(true)

		errorTextStyle = lipgloss.NewStyle().
				Foreground(deletedFg)

		resultTitleStyle = genericStyle.
					Foreground(textBg).
					Background(insertedFg).
					Bold(true)

		errorOverlayStyle = genericStyle.
					Border(lipgloss.RoundedBorder()).
					BorderForeground(deletedFg)

		genericOverlayStyle = genericStyle.
					Border(lipgloss.RoundedBorder())

		subtleStyle = lipgloss.NewStyle().
				Foreground(commentFg)

		insertedStyle = lipgloss.NewStyle().
				Foreground(textBg).
				Background(insertedFg)

		deletedStyle = lipgloss.NewStyle().
				Foreground(textBg).
				Background(deletedFg)

		paginationStyle = subtleStyle

		lineNumberStyle = subtleStyle
	)

	return &Theme{
		StatusBar: StatusBarStyles{
			Style:            statusBarStyle,
			PosStyle:         statusBarPosStyle,
			HelpStyle:        statusBarHelpStyle,
			MessageStyle:     statusBarMessageStyle,
			MessagePosStyle:  statusBarMessagePosStyle,
			MessageHelpStyle: statusBarMessageHelpStyle,
		},
		Error: ErrorStyles{
			OverlayStyle: errorOverlayStyle,
			TitleStyle:   errorTitleStyle,
			TextStyle:    errorTextStyle,
		},

		CursorStyle:         cursorStyle,
		ResultTitleStyle:    resultTitleStyle,
		FilterStyle:         filterStyle,
		GenericOverlayStyle: genericOverlayStyle,
		GenericTextStyle:    genericStyle,
		HelpStyle:           helpStyle,
		LineNumberStyle:     lineNumberStyle,
		LogoStyle:           logoStyle,
		PaginationStyle:     paginationStyle,
		SelectedStyle:       selectedStyle,
		SelectedSubtleStyle: selectedSubtleStyle,
		SubtleStyle:         subtleStyle,
		InsertedStyle:       insertedStyle,
		DeletedStyle:        deletedStyle,

		BackgroundColor: textBg,

		NiceyamlStyles: ss,
		Ellipsis:       Ellipsis,
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

func getStyleFg(ss style.Styles, s style.Style) color.Color {
	ls := ss.Style(s)
	if ls == nil {
		return lipgloss.NoColor{}
	}

	return ls.GetForeground()
}

func getStyleBg(ss style.Styles, s style.Style) color.Color {
	ls := ss.Style(s)
	if ls == nil {
		return lipgloss.NoColor{}
	}

	return ls.GetBackground()
}

// brighten adjusts a color towards a target by the given factor.
// For dark themes this lightens, for light themes this darkens.
func brighten(c, towards color.Color, factor float64) color.Color {
	if luminance(towards) > luminance(c) {
		return lipgloss.Lighten(c, factor)
	}

	return lipgloss.Darken(c, factor)
}

// luminance returns the relative luminance of a color per WCAG 2.0.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()

	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
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
