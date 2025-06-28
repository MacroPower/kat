package statusbar_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/themes"
)

func TestNewStatusBarRenderer(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		width    int
		expected int
	}{
		"positive width": {
			width:    80,
			expected: 80,
		},
		"zero width": {
			width:    0,
			expected: 30,
		},
		"negative width": {
			width:    -10,
			expected: 30,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := statusbar.NewStatusBarRenderer(themes.DefaultTheme, tc.width)
			require.NotNil(t, renderer)

			statusBar := renderer.RenderWithScroll("test", 0)
			assert.Len(t, statusBar, tc.expected)
		})
	}
}

func TestRenderStatusBar(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		checkFunc     func(*testing.T, string)
		statusMessage string
		title         string
		width         int
		scrollPercent float64
	}{
		"normal state": {
			width:         100,
			title:         "test-document",
			scrollPercent: 0.5,
			checkFunc: func(t *testing.T, result string) {
				t.Helper()
				// Should contain logo, document title, scroll percent, and help
				assert.Contains(t, result, "kat")           // Logo
				assert.Contains(t, result, "test-document") // Document title
				assert.Contains(t, result, "50%")           // Scroll percent
				assert.Contains(t, result, "? Help")        // Help note
			},
		},
		"status message state": {
			width:         100,
			statusMessage: "File saved successfully",
			title:         "test-document",
			scrollPercent: 0.75,
			checkFunc: func(t *testing.T, result string) {
				t.Helper()
				// Should contain logo, status message, scroll percent, and help
				assert.Contains(t, result, "kat")                     // Logo
				assert.Contains(t, result, "File saved successfully") // Status message
				assert.Contains(t, result, "75%")                     // Scroll percent
				assert.Contains(t, result, "? Help")                  // Help note
				assert.NotContains(t, result, "test-document")        // Should not contain doc title
			},
		},
		"narrow width": {
			width:         50,
			title:         "very-long-document-name-that-should-be-truncated",
			scrollPercent: 0.0,
			checkFunc: func(t *testing.T, result string) {
				t.Helper()
				// Should contain basic components but truncated
				assert.Contains(t, result, "kat")
				assert.Contains(t, result, "0%")
				assert.Contains(t, result, "? Help")
				// Should be truncated
				assert.Contains(t, result, "very-long-document-na…")
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := statusbar.NewStatusBarRenderer(themes.DefaultTheme, tc.width, statusbar.WithMessage(tc.statusMessage, statusbar.StyleSuccess))

			result := renderer.RenderWithScroll(tc.title, tc.scrollPercent)
			tc.checkFunc(t, result)

			// Verify the result is properly structured
			assert.NotEmpty(t, result)
		})
	}
}
