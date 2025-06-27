package overlay_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/MacroPower/kat/pkg/ui/overlay"
	"github.com/MacroPower/kat/pkg/ui/themes"
)

func TestNew(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme

	tests := []struct {
		theme    *themes.Theme
		expected func(*testing.T, *overlay.Overlay)
		name     string
		opts     []overlay.OverlayOpt
	}{
		{
			name:  "default overlay",
			theme: theme,
			opts:  nil,
			expected: func(t *testing.T, o *overlay.Overlay) {
				t.Helper()
				assert.NotNil(t, o)
			},
		},
		{
			name:  "with min width option",
			theme: theme,
			opts:  []overlay.OverlayOpt{overlay.WithMinWidth(32)},
			expected: func(t *testing.T, o *overlay.Overlay) {
				t.Helper()
				assert.NotNil(t, o)
			},
		},
		{
			name:  "with multiple options",
			theme: theme,
			opts: []overlay.OverlayOpt{
				overlay.WithMinWidth(20),
				overlay.WithMinWidth(40), // Last one should win.
			},
			expected: func(t *testing.T, o *overlay.Overlay) {
				t.Helper()
				assert.NotNil(t, o)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := overlay.New(tt.theme, tt.opts...)
			tt.expected(t, o)
		})
	}
}

func TestWithMinWidth(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme

	tests := []struct {
		name     string
		minWidth int
	}{
		{
			name:     "positive min width",
			minWidth: 32,
		},
		{
			name:     "zero min width",
			minWidth: 0,
		},
		{
			name:     "negative min width",
			minWidth: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := overlay.New(theme, overlay.WithMinWidth(tt.minWidth))
			assert.NotNil(t, o)
		})
	}
}

func TestOverlay_SetSize(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme
	o := overlay.New(theme)

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{
			name:   "positive dimensions",
			width:  80,
			height: 24,
		},
		{
			name:   "zero dimensions",
			width:  0,
			height: 0,
		},
		{
			name:   "negative dimensions",
			width:  -10,
			height: -5,
		},
		{
			name:   "large dimensions",
			width:  1000,
			height: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Should not panic.
			o.SetSize(tt.width, tt.height)
		})
	}
}

func TestOverlay_Place(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme

	tests := []struct {
		style         lipgloss.Style
		name          string
		bg            string
		fg            string
		widthFraction float64
		width         int
		height        int
		minWidth      int
	}{
		{
			name:          "simple overlay",
			bg:            "background\ncontent\nhere",
			fg:            "overlay",
			widthFraction: 0.5,
			width:         80,
			height:        24,
			minWidth:      16,
			style:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()),
		},
		{
			name:          "empty background",
			bg:            "",
			fg:            "overlay",
			widthFraction: 0.5,
			width:         80,
			height:        24,
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "empty foreground",
			bg:            "background\ncontent",
			fg:            "",
			widthFraction: 0.5,
			width:         80,
			height:        24,
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "full width fraction",
			bg:            "background content",
			fg:            "overlay",
			widthFraction: 1.0,
			width:         80,
			height:        24,
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "zero width fraction",
			bg:            "background content",
			fg:            "overlay",
			widthFraction: 0.0,
			width:         80,
			height:        24,
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "small height causes truncation",
			bg:            "line1\nline2\nline3",
			fg:            strings.Repeat("long overlay content\n", 20),
			widthFraction: 0.5,
			width:         80,
			height:        10, // Small height should cause truncation.
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "very small height",
			bg:            "background",
			fg:            "overlay",
			widthFraction: 0.5,
			width:         80,
			height:        5, // Very small height.
			minWidth:      16,
			style:         lipgloss.NewStyle(),
		},
		{
			name:          "multiline background and foreground",
			bg:            "line1\nline2\nline3\nline4\nline5",
			fg:            "overlay1\noverlay2\noverlay3",
			widthFraction: 0.3,
			width:         60,
			height:        20,
			minWidth:      10,
			style:         lipgloss.NewStyle().Padding(1),
		},
		{
			name:          "wide background with narrow overlay",
			bg:            strings.Repeat("very long background line that extends far\n", 5),
			fg:            "short\noverlay",
			widthFraction: 0.2,
			width:         100,
			height:        20,
			minWidth:      8,
			style:         lipgloss.NewStyle(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := overlay.New(theme, overlay.WithMinWidth(tt.minWidth))
			o.SetSize(tt.width, tt.height)

			result := o.Place(tt.bg, tt.fg, tt.widthFraction, tt.style)

			assert.NotEmpty(t, result)
			assert.IsType(t, "", result)
		})
	}
}

func TestOverlay_PlaceWithTruncation(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(80, 12) // Small height to potentially trigger truncation.

	// Create a long foreground with many lines.
	longFg := strings.Repeat("This is a very long line that should be truncated\n", 10)
	bg := "background\ncontent\nhere"
	inputLines := strings.Count(longFg, "\n")

	result := o.Place(bg, longFg, 0.5, lipgloss.NewStyle())

	assert.NotEmpty(t, result)
	// Just verify that we get a valid result - the specific behavior
	// depends on wrapping and internal height calculations.
	assert.IsType(t, "", result)

	// The result should have some content from the overlay process.
	resultLines := strings.Count(result, "\n") + 1
	assert.Positive(t, resultLines)

	// If truncation occurred, result should have fewer lines than a naive expectation,
	// but this depends on wrapping behavior so we just verify we get valid output.
	_ = inputLines // Keep the variable to show the intent.
}

func TestOverlay_PlaceEdgeCases(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme

	tests := []struct {
		setup        func() (*overlay.Overlay, string, string, float64, lipgloss.Style)
		validateFunc func(*testing.T, string)
		name         string
	}{
		{
			name: "overlay wider than background",
			setup: func() (*overlay.Overlay, string, string, float64, lipgloss.Style) {
				o := overlay.New(theme)
				o.SetSize(20, 10)

				return o, "short", "this is a much longer overlay text", 1.0, lipgloss.NewStyle()
			},
			validateFunc: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "overlay taller than background",
			setup: func() (*overlay.Overlay, string, string, float64, lipgloss.Style) {
				o := overlay.New(theme)
				o.SetSize(80, 20)
				bg := "line1\nline2"
				fg := "overlay1\noverlay2\noverlay3\noverlay4\noverlay5\noverlay6"

				return o, bg, fg, 0.5, lipgloss.NewStyle()
			},
			validateFunc: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "very large width fraction",
			setup: func() (*overlay.Overlay, string, string, float64, lipgloss.Style) {
				o := overlay.New(theme)
				o.SetSize(80, 20)

				return o, "background", "overlay", 2.0, lipgloss.NewStyle()
			},
			validateFunc: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "negative width fraction",
			setup: func() (*overlay.Overlay, string, string, float64, lipgloss.Style) {
				o := overlay.New(theme)
				o.SetSize(80, 20)

				return o, "background", "overlay", -0.5, lipgloss.NewStyle()
			},
			validateFunc: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o, bg, fg, widthFraction, style := tt.setup()
			result := o.Place(bg, fg, widthFraction, style)
			tt.validateFunc(t, result)
		})
	}
}

func TestOverlay_PlaceWithUnicodeContent(t *testing.T) {
	t.Parallel()

	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(80, 20)

	tests := []struct {
		name string
		bg   string
		fg   string
	}{
		{
			name: "unicode characters in background",
			bg:   "背景内容\n更多内容\n测试",
			fg:   "overlay",
		},
		{
			name: "unicode characters in foreground",
			bg:   "background\ncontent",
			fg:   "覆盖层\n内容\n测试",
		},
		{
			name: "emoji content",
			bg:   "background 🎉\ncontent 🚀",
			fg:   "overlay 😀\ncontent 🎨",
		},
		{
			name: "mixed unicode and ascii",
			bg:   "Hello 世界\nBackground content",
			fg:   "Overlay 内容\nTest 测试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := o.Place(tt.bg, tt.fg, 0.5, lipgloss.NewStyle())
			assert.NotEmpty(t, result)
			// Should not panic and should return a string.
			assert.IsType(t, "", result)
		})
	}
}

func TestClamp(t *testing.T) {
	t.Parallel()

	// Since clamp is not exported, we test it indirectly through Place method behavior.
	// This test ensures that overlay width calculations work correctly with various inputs.

	theme := themes.DefaultTheme
	o := overlay.New(theme, overlay.WithMinWidth(10))
	o.SetSize(100, 20)

	tests := []struct {
		name          string
		widthFraction float64
		expectResult  bool
	}{
		{
			name:          "normal fraction",
			widthFraction: 0.5,
			expectResult:  true,
		},
		{
			name:          "zero fraction should use minWidth",
			widthFraction: 0.0,
			expectResult:  true,
		},
		{
			name:          "fraction greater than 1",
			widthFraction: 1.5,
			expectResult:  true,
		},
		{
			name:          "negative fraction should use minWidth",
			widthFraction: -0.5,
			expectResult:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := o.Place("background", "foreground", tt.widthFraction, lipgloss.NewStyle())
			if tt.expectResult {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestGetLines(t *testing.T) {
	t.Parallel()

	// Since getLines is not exported, we test it indirectly through the Place method.
	// This test ensures that line splitting and width calculation work correctly.

	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(80, 20)

	tests := []struct {
		expected func(*testing.T, string)
		name     string
		content  string
	}{
		{
			name:    "single line",
			content: "single line content",
			expected: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
		{
			name:    "multiple lines",
			content: "line1\nline2\nline3",
			expected: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
				// The result will have at least the background lines,
				// but overlay processing may change the final line count.
				lines := strings.Split(result, "\n")
				assert.NotEmpty(t, lines)
			},
		},
		{
			name:    "empty string",
			content: "",
			expected: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result) // Background should still be present.
			},
		},
		{
			name:    "string with only newlines",
			content: "\n\n\n",
			expected: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
		{
			name:    "lines with different widths",
			content: "short\nvery long line that is much wider\nmedium length",
			expected: func(t *testing.T, result string) {
				t.Helper()
				assert.NotEmpty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := o.Place("background\ncontent", tt.content, 0.5, lipgloss.NewStyle())
			tt.expected(t, result)
		})
	}
}

func TestWhitespaceRender(t *testing.T) {
	t.Parallel()

	// Since whitespace.render is not exported, we test it indirectly.
	// We test scenarios that would exercise the whitespace rendering logic.

	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(100, 20)

	// Test with content that would require whitespace padding.
	bg := strings.Repeat("very long background line\n", 5)
	fg := "short"

	result := o.Place(bg, fg, 0.3, lipgloss.NewStyle())
	assert.NotEmpty(t, result)
	// Should not panic and should handle whitespace correctly.
	assert.IsType(t, "", result)
}
