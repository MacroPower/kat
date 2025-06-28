package statusbar_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/themes"
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

			kbr := &keys.KeyBindRenderer{}
			kbr.AddColumn(keys.NewBind("foo", keys.New("f")))

			renderer := statusbar.NewHelpRenderer(themes.DefaultTheme, kbr)
			require.NotNil(t, renderer)

			view := renderer.Render(tc.width)
			assert.NotEmpty(t, view, "Help view should not be empty")

			assert.Equal(t, 2, renderer.CalculateHelpHeight())
		})
	}
}

func TestHelpRenderer_GetHelpCommands(t *testing.T) {
	t.Parallel()

	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(ui.DefaultConfig.KeyBinds.Common.GetKeyBinds()...)
	kbr.AddColumn(ui.DefaultConfig.KeyBinds.Pager.GetKeyBinds()...)
	kbr.AddColumn(ui.DefaultConfig.KeyBinds.List.GetKeyBinds()...)

	helpView := kbr.Render(80)

	expectedCommands := []string{
		// "g/home",
		// "G/end",
		// "k/↑",
		// "j/↓",
		// "b/pgup",
		// "f/pgdn",
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

			kbr := &keys.KeyBindRenderer{}
			kbr.AddColumn(ui.DefaultConfig.KeyBinds.Common.GetKeyBinds()...)
			kbr.AddColumn(ui.DefaultConfig.KeyBinds.Pager.GetKeyBinds()...)
			kbr.AddColumn(ui.DefaultConfig.KeyBinds.List.GetKeyBinds()...)

			helpView := kbr.Render(tc.width)

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
