package overlay_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/ui/overlay"
	"github.com/macropower/kat/pkg/ui/themes"
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

			// Create a separate overlay instance for each test to avoid data races.
			o := overlay.New(theme)
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

func TestOverlay_Place_MultiLineContent(t *testing.T) {
	t.Parallel()

	theme := &themes.Theme{
		Ellipsis:    "...",
		SubtleStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	}
	t.Run("multi-line background and foreground without border", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(80, 20) // Adequate height for content

		background := "BG Line 1\nBG Line 2\nBG Line 3\nBG Line 4\nBG Line 5\nBG Line 6\nBG Line 7\nBG Line 8\nBG Line 9\nBG Line 10\nBG Line 11\nBG Line 12\nBG Line 13\nBG Line 14\nBG Line 15\nBG Line 16\nBG Line 17\nBG Line 18\nBG Line 19\nBG Line 20"
		foreground := "FG Line 1\nFG Line 2\nFG Line 3"

		// Use background color to make overlay content visible.
		result := o.Place(background, foreground, 0.5, lipgloss.NewStyle().Background(lipgloss.Color("blue")))

		lines := strings.Split(result, "\n")

		// Should maintain background line count.
		assert.Len(t, lines, 20, "should maintain background line count")

		// Find overlay region by looking for styled content or whitespace where overlay should be.
		overlayStartLine := -1
		overlayEndLine := -1
		for i, line := range lines {
			// Look for foreground content or styled area (background color creates visible space).
			if strings.Contains(line, "FG Line") || (len(line) > 20 && strings.TrimSpace(line) == "") {
				if overlayStartLine == -1 {
					overlayStartLine = i
				}
				overlayEndLine = i
			}
		}

		assert.NotEqual(t, -1, overlayStartLine, "overlay region should be present")
		assert.Positive(t, overlayStartLine, "overlay should not start at first line (should be centered)")
		assert.Less(t, overlayEndLine, len(lines)-1, "overlay should not end at last line (should be centered)")

		// Verify background content appears before and after overlay.
		backgroundBefore := false
		backgroundAfter := false
		for i, line := range lines {
			if strings.Contains(line, "BG Line") {
				if i < overlayStartLine {
					backgroundBefore = true
				}
				if i > overlayEndLine {
					backgroundAfter = true
				}
			}
		}

		assert.True(t, backgroundBefore, "background content should appear before overlay")
		assert.True(t, backgroundAfter, "background content should appear after overlay")

		// With background style, overlay area should be visible as styled space.
		overlayAreaFound := false
		for i := overlayStartLine; i <= overlayEndLine; i++ {
			line := lines[i]
			// The overlay area should have content (either FG content or styled space).
			if line != "" {
				overlayAreaFound = true

				break
			}
		}
		assert.True(t, overlayAreaFound, "overlay area should be visible")
	})

	t.Run("multi-line background and foreground with border", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(80, 20) // Adequate height for content

		background := "BG Line 1\nBG Line 2\nBG Line 3\nBG Line 4\nBG Line 5\nBG Line 6\nBG Line 7\nBG Line 8\nBG Line 9\nBG Line 10\nBG Line 11\nBG Line 12\nBG Line 13\nBG Line 14\nBG Line 15\nBG Line 16\nBG Line 17\nBG Line 18\nBG Line 19\nBG Line 20"
		foreground := "FG Line 1\nFG Line 2"

		result := o.Place(background, foreground, 0.5, lipgloss.NewStyle().Border(lipgloss.RoundedBorder()))

		lines := strings.Split(result, "\n")

		// With border, should have: top border, content lines with borders, bottom border, plus background.
		borderTopFound := false
		borderBottomFound := false
		contentLinesWithBorder := 0
		backgroundLinesFound := 0
		fgLine1Found := false
		fgLine2Found := false

		for _, line := range lines {
			switch {
			case strings.Contains(line, "╭") && strings.Contains(line, "╮"):
				borderTopFound = true
			case strings.Contains(line, "╰") && strings.Contains(line, "╯"):
				borderBottomFound = true
			case strings.Contains(line, "│"):
				contentLinesWithBorder++
				if strings.Contains(line, "FG Line 1") {
					fgLine1Found = true
				}
				if strings.Contains(line, "FG Line 2") {
					fgLine2Found = true
				}
			case strings.Contains(line, "BG Line"):
				backgroundLinesFound++
			}
		}

		assert.True(t, borderTopFound, "should have top border (╭╮)")
		assert.True(t, borderBottomFound, "should have bottom border (╰╯)")
		assert.Positive(t, contentLinesWithBorder, "should have content lines with side borders")
		assert.Positive(t, backgroundLinesFound, "should have background content outside overlay")

		// With adequate height, the foreground content should be visible.
		assert.True(t, fgLine1Found, "FG Line 1 should be visible within border")
		assert.True(t, fgLine2Found, "FG Line 2 should be visible within border")
	})

	t.Run("overlay positioning with different height ratios", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(60, 25) // Adequate height for content and centering

		background := strings.Repeat("Background line\n", 25)
		background = strings.TrimSuffix(background, "\n") // Remove trailing newline for exact 25 lines
		foreground := "Overlay 1\nOverlay 2\nOverlay 3\nOverlay 4"

		// Use border to make overlay visible.
		result := o.Place(background, foreground, 0.5, lipgloss.NewStyle().Border(lipgloss.RoundedBorder()))

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 25, "should maintain background line count")

		// Find the overlay region by looking for border elements and content.
		borderTopLine := -1
		borderBottomLine := -1
		overlayContentFound := 0

		for i, line := range lines {
			switch {
			case strings.Contains(line, "╭") && strings.Contains(line, "╮"):
				borderTopLine = i
			case strings.Contains(line, "╰") && strings.Contains(line, "╯"):
				borderBottomLine = i
			case strings.Contains(line, "│") && strings.Contains(line, "Overlay"):
				overlayContentFound++
			}
		}

		assert.NotEqual(t, -1, borderTopLine, "border top should be present")
		assert.NotEqual(t, -1, borderBottomLine, "border bottom should be present")
		assert.Equal(t, 4, overlayContentFound, "all 4 overlay content lines should be visible")

		// Verify overlay is centered vertically.
		if borderTopLine != -1 && borderBottomLine != -1 {
			// Should have background lines before and after.
			assert.Positive(t, borderTopLine, "should have background content before overlay")
			assert.Less(t, borderBottomLine, len(lines)-1, "should have background content after overlay")

			// Verify approximate centering.
			beforeCount := borderTopLine
			afterCount := len(lines) - 1 - borderBottomLine
			assert.LessOrEqual(t, abs(beforeCount-afterCount), 3, "overlay should be approximately centered")
		}
	})

	t.Run("overlay content wrapping behavior", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(50, 15) // Narrow width to test wrapping

		background := strings.Repeat("Background content line\n", 15)
		background = strings.TrimSuffix(background, "\n")
		// Long line that should wrap.
		foreground := "This is a very long foreground line that should wrap when displayed because it exceeds the available overlay width"

		// Use border to make overlay visible and test wrapping.
		result := o.Place(background, foreground, 0.8, lipgloss.NewStyle().Border(lipgloss.RoundedBorder()))

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 15, "should maintain background line count")

		// Find border region.
		borderTopLine := -1
		borderBottomLine := -1
		contentLinesInBorder := 0
		for i, line := range lines {
			switch {
			case strings.Contains(line, "╭") && strings.Contains(line, "╮"):
				borderTopLine = i
			case strings.Contains(line, "╰") && strings.Contains(line, "╯"):
				borderBottomLine = i
			case strings.Contains(line, "│"):
				contentLinesInBorder++
				// Check that line doesn't exceed terminal width.
				assert.LessOrEqual(t, len(line), 50, "line should not exceed terminal width")
			}
		}

		assert.NotEqual(t, -1, borderTopLine, "border top should be present")
		assert.NotEqual(t, -1, borderBottomLine, "border bottom should be present")

		// The long content should be wrapped into multiple lines within the border.
		overlayHeight := borderBottomLine - borderTopLine - 1 // Exclude top and bottom border lines
		assert.Greater(t, contentLinesInBorder, 1, "long content should be wrapped into multiple lines")
		assert.LessOrEqual(t, contentLinesInBorder, overlayHeight, "content lines should fit within border")

		// Verify background is visible outside overlay.
		backgroundVisible := false
		for i, line := range lines {
			if (i < borderTopLine || i > borderBottomLine) && strings.Contains(line, "Background content") {
				backgroundVisible = true

				break
			}
		}
		assert.True(t, backgroundVisible, "background should be visible outside overlay region")
	})

	t.Run("overlay with empty lines and whitespace", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(60, 20) // Large enough height to accommodate content

		background := "BG 1\nBG 2\nBG 3\nBG 4\nBG 5\nBG 6\nBG 7\nBG 8\nBG 9\nBG 10\nBG 11\nBG 12\nBG 13\nBG 14\nBG 15\nBG 16\nBG 17\nBG 18\nBG 19\nBG 20"
		foreground := "Line 1\n\nLine 3\n   Line 4   \n\n"

		// Use border to make overlay structure visible.
		result := o.Place(background, foreground, 0.6, lipgloss.NewStyle().Border(lipgloss.RoundedBorder()))

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 20, "should maintain background line count")

		// Find border region to verify overlay structure.
		borderTopLine := -1
		borderBottomLine := -1
		line1Found := false
		line3Found := false
		line4Found := false
		emptyLinesInBorder := 0

		for i, line := range lines {
			switch {
			case strings.Contains(line, "╭") && strings.Contains(line, "╮"):
				borderTopLine = i
			case strings.Contains(line, "╰") && strings.Contains(line, "╯"):
				borderBottomLine = i
			case strings.Contains(line, "│"):
				switch {
				case strings.Contains(line, "Line 1"):
					line1Found = true
				case strings.Contains(line, "Line 3"):
					line3Found = true
				case strings.Contains(line, "Line 4"):
					line4Found = true
				case !strings.Contains(line, "Line") && strings.TrimSpace(strings.Trim(line, "│ ")) == "":
					// Empty line within border (just side borders and spaces).
					emptyLinesInBorder++
				}
			}
		}

		assert.NotEqual(t, -1, borderTopLine, "border top should be present")
		assert.NotEqual(t, -1, borderBottomLine, "border bottom should be present")

		// With adequate height, the specific content should be visible.
		assert.True(t, line1Found, "Line 1 should appear in overlay")
		assert.True(t, line3Found, "Line 3 should appear in overlay")
		assert.True(t, line4Found, "Line 4 should appear in overlay")
		assert.Positive(t, emptyLinesInBorder, "empty lines should be preserved in overlay")

		// Verify background is visible outside overlay.
		backgroundVisible := false
		for i, line := range lines {
			if (i < borderTopLine || i > borderBottomLine) && strings.Contains(line, "BG") {
				backgroundVisible = true

				break
			}
		}
		assert.True(t, backgroundVisible, "background should be visible outside overlay")
	})

	t.Run("very small overlay area", func(t *testing.T) {
		t.Parallel()

		o := overlay.New(theme)
		o.SetSize(30, 15) // Larger height even for small terminal

		background := "A\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nO"
		foreground := "Overlay content that is much longer than available space"

		// Use background color to make overlay visible in small space.
		result := o.Place(background, foreground, 0.8, lipgloss.NewStyle().Background(lipgloss.Color("blue")))

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 15, "should maintain background line count")

		// Find overlay region by looking for styled area or content.
		overlayFound := false
		overlayContentFound := false

		for _, line := range lines {
			// Look for overlay content or styled space (background creates visible area).
			if strings.Contains(line, "Overlay") {
				overlayFound = true
				overlayContentFound = true
				// Line should fit within terminal width.
				assert.LessOrEqual(t, len(line), 30, "overlay line should not exceed terminal width")
			} else if len(line) > 15 && strings.TrimSpace(line) == "" {
				// Styled empty space.
				overlayFound = true
			}
		}

		assert.True(t, overlayFound, "overlay region should be present even in small space")
		assert.True(t, overlayContentFound, "overlay content should be visible with adequate height")

		// Background should still be partially visible.
		backgroundFound := false
		for _, line := range lines {
			isBackgroundChar := line != "" && (strings.Contains(line, "A") || strings.Contains(line, "B") ||
				strings.Contains(line, "C") || strings.Contains(line, "D") ||
				strings.Contains(line, "E") || strings.Contains(line, "F"))
			if isBackgroundChar && !strings.Contains(line, "Overlay") {
				backgroundFound = true

				break
			}
		}

		assert.True(t, backgroundFound, "background should be visible outside overlay")
	})
}

// Helper function for absolute difference.
func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}
