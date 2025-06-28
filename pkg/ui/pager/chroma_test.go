package pager_test

import (
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/MacroPower/kat/pkg/ui/pager"
	"github.com/MacroPower/kat/pkg/ui/themes"
)

func TestChromaRenderer(t *testing.T) {
	t.Parallel()

	// Force lipgloss to use a specific renderer profile.
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a basic theme for testing.
	theme := &themes.Theme{
		SelectedStyle:   lipgloss.NewStyle().Background(lipgloss.Color("12")),
		LineNumberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		ChromaStyle:     styles.Get("github"),
	}

	// Create a ChromaRenderer instance with the theme.
	renderer := pager.NewChromaRenderer(theme, true)
	renderer.SetFormatter("terminal16m")

	// Add tests here.
}
