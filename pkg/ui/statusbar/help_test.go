package statusbar_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
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

			renderer := statusbar.NewHelpRenderer(theme.Default, kbr)
			require.NotNil(t, renderer)

			view := renderer.Render(tc.width)
			assert.NotEmpty(t, view, "Help view should not be empty")

			assert.Equal(t, 2, renderer.CalculateHelpHeight(tc.width))
		})
	}
}

func TestHelpRenderer_Render(t *testing.T) {
	t.Parallel()

	cfg := ui.NewConfig()

	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(cfg.KeyBinds.Common.GetKeyBinds()...)
	kbr.AddColumn(cfg.KeyBinds.Pager.GetKeyBinds()...)
	kbr.AddColumn(cfg.KeyBinds.List.GetKeyBinds()...)

	tcs := map[string]struct {
		width int
	}{
		"width 80": {width: 80},
		"width 40": {width: 40},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := statusbar.NewHelpRenderer(theme.Default, kbr)
			result := renderer.Render(tc.width)
			golden.RequireEqual(t, result)
		})
	}
}

func TestHelpRenderer_GetHelpCommands(t *testing.T) {
	t.Parallel()

	cfg := ui.NewConfig()

	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(cfg.KeyBinds.Common.GetKeyBinds()...)
	kbr.AddColumn(cfg.KeyBinds.Pager.GetKeyBinds()...)
	kbr.AddColumn(cfg.KeyBinds.List.GetKeyBinds()...)

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

			cfg := ui.NewConfig()

			kbr := &keys.KeyBindRenderer{}
			kbr.AddColumn(cfg.KeyBinds.Common.GetKeyBinds()...)
			kbr.AddColumn(cfg.KeyBinds.Pager.GetKeyBinds()...)
			kbr.AddColumn(cfg.KeyBinds.List.GetKeyBinds()...)

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

func TestHelpModel_View(t *testing.T) {
	t.Parallel()

	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(keys.NewBind("foo", keys.New("f")))
	kbr.AddColumn(keys.NewBind("bar", keys.New("b")))

	renderer := statusbar.NewHelpRenderer(theme.Default, kbr)

	t.Run("visible", func(t *testing.T) {
		t.Parallel()

		model := statusbar.NewHelpModel(renderer)
		model.SetWidth(80)
		model.SetVisible(true)

		result := model.View(80)
		golden.RequireEqual(t, result)
	})

	t.Run("not visible", func(t *testing.T) {
		t.Parallel()

		model := statusbar.NewHelpModel(renderer)
		model.SetWidth(80)

		result := model.View(80)
		assert.Empty(t, result)
	})
}
