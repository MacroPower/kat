package theme_test

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.jacobcolvin.com/niceyaml/style"

	"github.com/macropower/kat/pkg/ui/theme"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err    error
		styles style.Styles
		name   string
	}{
		"successful registration with valid styles": {
			name: "test-theme-1",
			styles: style.NewStyles(
				lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#000000")),
				style.Set(style.Comment, lipgloss.NewStyle().Foreground(lipgloss.Color("#008000"))),
				style.Set(style.NameTag, lipgloss.NewStyle().Foreground(lipgloss.Color("#800080")).Bold(true)),
			),
			err: nil,
		},
		"successful registration with minimal styles": {
			name: "minimal-theme-1",
			styles: style.NewStyles(
				lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
			),
			err: nil,
		},
		"registration with empty name": {
			name: "",
			styles: style.NewStyles(
				lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")),
			),
			err: theme.ErrInvalidName,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := theme.Register(tc.name, tc.styles)
			if tc.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)

			// Verify the theme was registered by creating a new theme with it.
			if tc.name != "" {
				newTheme := theme.New(tc.name)
				assert.NotNil(t, newTheme)
				assert.NotNil(t, newTheme.NiceyamlStyles)
			}
		})
	}
}

func TestTheme_StylesRenderContent(t *testing.T) {
	t.Parallel()

	themeInstance := theme.New("github")
	testContent := "test content"

	tcs := map[string]struct {
		style lipgloss.Style
	}{
		"CursorStyle": {
			style: themeInstance.CursorStyle,
		},
		"Error.OverlayStyle": {
			style: themeInstance.Error.OverlayStyle,
		},
		"Error.TitleStyle": {
			style: themeInstance.Error.TitleStyle,
		},
		"ResultTitleStyle": {
			style: themeInstance.ResultTitleStyle,
		},
		"FilterStyle": {
			style: themeInstance.FilterStyle,
		},
		"GenericOverlayStyle": {
			style: themeInstance.GenericOverlayStyle,
		},
		"GenericTextStyle": {
			style: themeInstance.GenericTextStyle,
		},
		"HelpStyle": {
			style: themeInstance.HelpStyle,
		},
		"LineNumberStyle": {
			style: themeInstance.LineNumberStyle,
		},
		"LogoStyle": {
			style: themeInstance.LogoStyle,
		},
		"PaginationStyle": {
			style: themeInstance.PaginationStyle,
		},
		"SelectedStyle": {
			style: themeInstance.SelectedStyle,
		},
		"SelectedSubtleStyle": {
			style: themeInstance.SelectedSubtleStyle,
		},
		"StatusBar.HelpStyle": {
			style: themeInstance.StatusBar.HelpStyle,
		},
		"StatusBar.MessageHelpStyle": {
			style: themeInstance.StatusBar.MessageHelpStyle,
		},
		"StatusBar.MessagePosStyle": {
			style: themeInstance.StatusBar.MessagePosStyle,
		},
		"StatusBar.MessageStyle": {
			style: themeInstance.StatusBar.MessageStyle,
		},
		"StatusBar.PosStyle": {
			style: themeInstance.StatusBar.PosStyle,
		},
		"StatusBar.Style": {
			style: themeInstance.StatusBar.Style,
		},
		"SubtleStyle": {
			style: themeInstance.SubtleStyle,
		},
		"InsertedStyle": {
			style: themeInstance.InsertedStyle,
		},
		"DeletedStyle": {
			style: themeInstance.DeletedStyle,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rendered := tc.style.Render(testContent)

			// Verify that the style can render content without panicking
			// and produces some output.
			assert.NotEmpty(t, rendered)
			// The rendered content should contain the original content.
			assert.Contains(t, rendered, testContent)
		})
	}
}

func TestTheme_DifferentThemesProduceDifferentStyles(t *testing.T) {
	t.Parallel()

	lightTheme := theme.New("light")
	darkTheme := theme.New("dark")

	// The themes should be different objects.
	assert.NotEqual(t, lightTheme, darkTheme)

	// They should have different styles.
	assert.NotEqual(t, lightTheme.GenericTextStyle.Render("x"), darkTheme.GenericTextStyle.Render("x"))
}
