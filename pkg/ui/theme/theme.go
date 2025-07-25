package theme

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Icons.
const (
	Ellipsis = "…"
)

var (
	Default = New(getDefaultStyle())

	ErrRegisterStyles = errors.New("register theme styles")
	ErrInvalidName    = errors.New("invalid theme name")

	// Protect chroma styles from concurrent access.
	chromaStyleMutex sync.Mutex
)

type Theme struct {
	CursorStyle               lipgloss.Style
	ErrorOverlayStyle         lipgloss.Style
	ErrorTitleStyle           lipgloss.Style
	ErrorTextStyle            lipgloss.Style
	ResultTitleStyle          lipgloss.Style
	FilterStyle               lipgloss.Style
	GenericOverlayStyle       lipgloss.Style
	GenericTextStyle          lipgloss.Style
	HelpStyle                 lipgloss.Style
	LineNumberStyle           lipgloss.Style
	LogoStyle                 lipgloss.Style
	PaginationStyle           lipgloss.Style
	SelectedStyle             lipgloss.Style
	SelectedSubtleStyle       lipgloss.Style
	StatusBarHelpStyle        lipgloss.Style
	StatusBarMessageHelpStyle lipgloss.Style
	StatusBarMessagePosStyle  lipgloss.Style
	StatusBarMessageStyle     lipgloss.Style
	StatusBarPosStyle         lipgloss.Style
	StatusBarStyle            lipgloss.Style
	SubtleStyle               lipgloss.Style
	InsertedStyle             lipgloss.Style
	DeletedStyle              lipgloss.Style

	ChromaStyle *chroma.Style
	Ellipsis    string
}

func New(theme string) *Theme {
	style := newChromaStyle(theme)

	var (
		genericStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromToken(chroma.Background))

		logoStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromTokenBg(chroma.Background)).
				Background(style.lipglossFromToken(chroma.NameTag)).
				Bold(true)

		selectedStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromToken(chroma.NameTag))

		selectedSubtleStyle = lipgloss.NewStyle().
					Foreground(style.lipglossFromTokenWithFactor(chroma.NameTag, 0.3))

		filterStyle = selectedStyle

		cursorStyle = selectedSubtleStyle

		helpStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromTokenWithFactor(chroma.Background, 0.2)).
				Background(style.lipglossFromTokenBgWithFactor(chroma.Background, 0.2))

		statusBarStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromToken(chroma.Background)).
				Background(style.lipglossFromTokenBgWithFactor(chroma.Background, 0.1))

		statusBarPosStyle = lipgloss.NewStyle().
					Foreground(style.lipglossFromToken(chroma.Background)).
					Background(style.lipglossFromTokenBgWithFactor(chroma.Background, 0.15))

		statusBarHelpStyle = helpStyle

		statusBarMessageStyle = lipgloss.NewStyle().
					Foreground(style.lipglossFromTokenBg(chroma.Background)).
					Background(style.lipglossFromTokenWithFactor(chroma.NameTag, 0.15))

		statusBarMessagePosStyle = lipgloss.NewStyle().
						Foreground(style.lipglossFromTokenBg(chroma.Background)).
						Background(style.lipglossFromTokenWithFactor(chroma.NameTag, 0.1))

		statusBarMessageHelpStyle = genericStyle.
						Foreground(style.lipglossFromTokenBg(chroma.Background)).
						Background(style.lipglossFromToken(chroma.NameTag))

		errorTitleStyle = genericStyle.
				Foreground(style.lipglossFromTokenBg(chroma.Background)).
				Background(style.lipglossFromToken(chroma.GenericDeleted)).
				Bold(true)

		errorTextStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromToken(chroma.GenericDeleted))

		resultTitleStyle = genericStyle.
					Foreground(style.lipglossFromTokenBg(chroma.Background)).
					Background(style.lipglossFromToken(chroma.GenericInserted)).
					Bold(true)

		errorOverlayStyle = genericStyle.
					Border(lipgloss.RoundedBorder()).
					BorderForeground(style.lipglossFromToken(chroma.GenericDeleted))

		genericOverlayStyle = genericStyle.
					Border(lipgloss.RoundedBorder())

		subtleStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromToken(chroma.Comment))

		insertedStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromTokenBg(chroma.Background)).
				Background(style.lipglossFromToken(chroma.GenericInserted))

		deletedStyle = lipgloss.NewStyle().
				Foreground(style.lipglossFromTokenBg(chroma.Background)).
				Background(style.lipglossFromToken(chroma.GenericDeleted))

		paginationStyle = subtleStyle

		lineNumberStyle = subtleStyle
	)

	return &Theme{
		CursorStyle:               cursorStyle,
		ErrorOverlayStyle:         errorOverlayStyle,
		ErrorTitleStyle:           errorTitleStyle,
		ErrorTextStyle:            errorTextStyle,
		ResultTitleStyle:          resultTitleStyle,
		FilterStyle:               filterStyle,
		GenericOverlayStyle:       genericOverlayStyle,
		GenericTextStyle:          genericStyle,
		HelpStyle:                 helpStyle,
		LineNumberStyle:           lineNumberStyle,
		LogoStyle:                 logoStyle,
		PaginationStyle:           paginationStyle,
		SelectedStyle:             selectedStyle,
		SelectedSubtleStyle:       selectedSubtleStyle,
		StatusBarHelpStyle:        statusBarHelpStyle,
		StatusBarMessageHelpStyle: statusBarMessageHelpStyle,
		StatusBarMessagePosStyle:  statusBarMessagePosStyle,
		StatusBarMessageStyle:     statusBarMessageStyle,
		StatusBarPosStyle:         statusBarPosStyle,
		StatusBarStyle:            statusBarStyle,
		SubtleStyle:               subtleStyle,
		InsertedStyle:             insertedStyle,
		DeletedStyle:              deletedStyle,

		ChromaStyle: style.style,
		Ellipsis:    Ellipsis,
	}
}

func Register(name string, entries chroma.StyleEntries) error {
	if name == "" {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}

	chromaStyleMutex.Lock()
	defer chromaStyleMutex.Unlock()

	customTheme, err := chroma.NewStyle(name, entries)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRegisterStyles, err)
	}

	styles.Register(customTheme)

	return nil
}

type chromaStyle struct {
	style *chroma.Style
}

func newChromaStyle(theme string) chromaStyle {
	chromaStyleMutex.Lock()
	defer chromaStyleMutex.Unlock()

	s := styles.Get(getStyle(theme))
	if s == nil {
		// If the style is not found, fallback to the default style.
		s = styles.Fallback
	}

	return chromaStyle{
		style: s,
	}
}

func (cs chromaStyle) lipglossFromToken(c chroma.TokenType) lipgloss.Color {
	s := cs.style.Get(c)

	// Convert chroma color to lipgloss.AdaptiveColor.
	return lipgloss.Color(s.Colour.String()) //nolint:misspell // Chroma naming.
}

//nolint:unparam // Will you shut up man...
func (cs chromaStyle) lipglossFromTokenBg(c chroma.TokenType) lipgloss.Color {
	s := cs.style.Get(c)

	// Convert chroma color to lipgloss.AdaptiveColor.
	return lipgloss.Color(s.Background.String())
}

func (cs chromaStyle) lipglossFromTokenWithFactor(c chroma.TokenType, factor float64) lipgloss.Color {
	s := cs.style.Get(c)

	sc := s.Colour.BrightenOrDarken(factor) //nolint:misspell // Chroma naming.

	// Convert chroma color to lipgloss.AdaptiveColor.
	return lipgloss.Color(sc.String())
}

func (cs chromaStyle) lipglossFromTokenBgWithFactor(c chroma.TokenType, factor float64) lipgloss.Color {
	s := cs.style.Get(c)

	sc := s.Background.BrightenOrDarken(factor)

	// Convert chroma color to lipgloss.AdaptiveColor.
	return lipgloss.Color(sc.String())
}

func getStyle(style string) string {
	switch style {
	case "dark":
		return "github-dark"
	case "light":
		return "github"
	case "auto", "":
		return getDefaultStyle()
	default:
		return style
	}
}

func getDefaultStyle() string {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return "" // Fallback.
	}
	if termenv.HasDarkBackground() {
		return "github-dark"
	}

	return "github"
}
