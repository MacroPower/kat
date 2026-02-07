package statusbar_test

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
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
			expected: 27,
		},
		"negative width": {
			width:    -10,
			expected: 27,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := statusbar.NewStatusBarRenderer(theme.Default, tc.width)
			require.NotNil(t, renderer)

			statusBar := renderer.RenderWithScroll("test", 0)
			assert.Equal(t, tc.expected, lipgloss.Width(statusBar))
		})
	}
}

func TestRenderStatusBar(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		statusMessage string
		title         string
		width         int
		scrollPercent float64
	}{
		"normal state": {
			width:         100,
			title:         "test-document",
			scrollPercent: 0.5,
		},
		"status message state": {
			width:         100,
			statusMessage: "File saved successfully",
			title:         "test-document",
			scrollPercent: 0.75,
		},
		"narrow width": {
			width:         50,
			title:         "very-long-document-name-that-should-be-truncated",
			scrollPercent: 0.0,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := []statusbar.StatusBarOpt{}
			if tc.statusMessage != "" {
				opts = append(opts, statusbar.WithMessage(tc.statusMessage, statusbar.StyleSuccess))
			}

			renderer := statusbar.NewStatusBarRenderer(
				theme.Default,
				tc.width,
				opts...,
			)

			result := renderer.RenderWithScroll(tc.title, tc.scrollPercent)
			golden.RequireEqual(t, result)
		})
	}
}

func TestRenderStatusBarWithNote(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		statusMessage string
		title         string
		progress      string
		width         int
	}{
		"normal state with progress": {
			width:    100,
			title:    "test-document",
			progress: "3/10",
		},
		"status message with progress": {
			width:         100,
			statusMessage: "File saved successfully",
			title:         "test-document",
			progress:      "7/10",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := []statusbar.StatusBarOpt{}
			if tc.statusMessage != "" {
				opts = append(opts, statusbar.WithMessage(tc.statusMessage, statusbar.StyleSuccess))
			}

			renderer := statusbar.NewStatusBarRenderer(
				theme.Default,
				tc.width,
				opts...,
			)

			result := renderer.RenderWithNote(tc.title, tc.progress)
			golden.RequireEqual(t, result)
		})
	}
}
