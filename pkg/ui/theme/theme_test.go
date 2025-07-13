package theme_test

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/theme"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err     error
		entries chroma.StyleEntries
		name    string
	}{
		"successful registration with valid entries": {
			name: "test-theme-1",
			entries: chroma.StyleEntries{
				chroma.Background:      "#ffffff #000000",
				chroma.Comment:         "italic #008000",
				chroma.Keyword:         "bold #0000ff",
				chroma.String:          "#008000",
				chroma.Number:          "#ff0000",
				chroma.NameTag:         "bold #800080",
				chroma.GenericInserted: "#000000 #00ff00",
				chroma.GenericDeleted:  "#000000 #ff0000",
			},
			err: nil,
		},
		"successful registration with minimal entries": {
			name: "minimal-theme-1",
			entries: chroma.StyleEntries{
				chroma.Background: "#ffffff #000000",
			},
			err: nil,
		},
		"registration with empty name": {
			name: "",
			entries: chroma.StyleEntries{
				chroma.Background: "#ffffff #000000",
			},
			err: theme.ErrInvalidName,
		},
		"invalid color format": {
			name: "invalid-color-theme-1",
			entries: chroma.StyleEntries{
				chroma.Background: "invalid-color-format",
			},
			err: theme.ErrRegisterStyles,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := theme.Register(tc.name, tc.entries)
			if tc.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.err)

				return
			}

			require.NoError(t, err)

			// Verify the theme was registered by creating a new theme with it
			if tc.name != "" {
				newTheme := theme.New(tc.name)
				assert.NotNil(t, newTheme)
				assert.NotNil(t, newTheme.ChromaStyle)
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
		"ErrorOverlayStyle": {
			style: themeInstance.ErrorOverlayStyle,
		},
		"ErrorTitleStyle": {
			style: themeInstance.ErrorTitleStyle,
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
		"StatusBarHelpStyle": {
			style: themeInstance.StatusBarHelpStyle,
		},
		"StatusBarMessageHelpStyle": {
			style: themeInstance.StatusBarMessageHelpStyle,
		},
		"StatusBarMessagePosStyle": {
			style: themeInstance.StatusBarMessagePosStyle,
		},
		"StatusBarMessageStyle": {
			style: themeInstance.StatusBarMessageStyle,
		},
		"StatusBarPosStyle": {
			style: themeInstance.StatusBarPosStyle,
		},
		"StatusBarStyle": {
			style: themeInstance.StatusBarStyle,
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
			// and produces some output
			assert.NotEmpty(t, rendered)
			// The rendered content should contain the original content
			assert.Contains(t, rendered, testContent)
		})
	}
}

func TestTheme_DifferentThemesProduceDifferentStyles(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	lightTheme := theme.New("light")
	darkTheme := theme.New("dark")

	// The themes should be different objects
	assert.NotEqual(t, lightTheme, darkTheme)

	// They should have different chroma styles
	assert.NotEqual(t, lightTheme.GenericTextStyle.Render("x"), darkTheme.GenericTextStyle.Render("x"))
}
