package pager_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/ui/pager"
)

func TestNewHelpRenderer(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		width int
	}{
		"positive width": {
			width: 80,
		},
		"zero width": {
			width: 0,
		},
		"negative width": {
			width: -10,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := pager.NewHelpRenderer(tc.width)
			require.NotNil(t, renderer)
		})
	}
}

func TestHelpRenderer_RenderHelpView(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		width int
	}{
		"standard width": {
			width: 80,
		},
		"narrow width": {
			width: 40,
		},
		"zero width": {
			width: 0,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := pager.NewHelpRenderer(tc.width)
			helpView := renderer.RenderHelpView()

			assert.NotEmpty(t, helpView)
			assert.Contains(t, helpView, "up")
			assert.Contains(t, helpView, "down")
			assert.Contains(t, helpView, "go to top")
			assert.Contains(t, helpView, "go to bottom")
			assert.Contains(t, helpView, "copy contents")
			assert.Contains(t, helpView, "reload this document")
			assert.Contains(t, helpView, "back to files")
			assert.Contains(t, helpView, "quit")
		})
	}
}

func TestHelpRenderer_CalculateHelpHeight(t *testing.T) {
	t.Parallel()

	renderer := pager.NewHelpRenderer(80)
	height := renderer.CalculateHelpHeight()

	assert.Positive(t, height)
	assert.Equal(t, height, strings.Count(renderer.RenderHelpView(), "\n"))
}

func TestHelpRenderer_GetHelpCommands(t *testing.T) {
	t.Parallel()

	renderer := pager.NewHelpRenderer(80)

	// Test that the help content contains expected commands
	helpView := renderer.RenderHelpView()

	expectedCommands := []string{
		"g/home",
		"G/end",
		"k/↑",
		"j/↓",
		"b/pgup",
		"f/pgdn",
		"u",
		"d",
		"c",
		"r",
		"esc",
		"q",
	}

	for _, cmd := range expectedCommands {
		assert.Contains(t, helpView, cmd, "Help view should contain command: %s", cmd)
	}
}

func TestHelpRenderer_FillEmptySpaces(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		width     int
		hasSpaces bool
	}{
		"positive width should fill spaces": {
			width:     80,
			hasSpaces: true,
		},
		"zero width should not fill spaces": {
			width:     0,
			hasSpaces: false,
		},
		"negative width should not fill spaces": {
			width:     -10,
			hasSpaces: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := pager.NewHelpRenderer(tc.width)
			helpView := renderer.RenderHelpView()

			if tc.hasSpaces && tc.width > 0 {
				// Should contain trailing spaces for background coloring
				lines := strings.SplitSeq(helpView, "\n")
				for line := range lines {
					if line != "" {
						// At least some lines should have trailing spaces
						assert.GreaterOrEqual(t, len(line), 10, "Lines should be padded for background coloring")
					}
				}
			}
		})
	}
}
