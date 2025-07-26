package cli

import (
	"image/color"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"

	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/ui/theme"
)

// Try to get the theme from the config, otherwise use the default color scheme.
func ColorSchemeFunc(c lipgloss.LightDarkFunc) fang.ColorScheme {
	configPath := config.GetPath()

	cl, err := config.NewConfigLoaderFromFile(configPath, config.WithThemeFromData())
	if err != nil {
		return ThemeColorScheme(theme.Default, c)
	}

	return ThemeColorScheme(cl.GetTheme(), c)
}

func ThemeColorScheme(t *theme.Theme, c lipgloss.LightDarkFunc) fang.ColorScheme {
	return fang.ColorScheme{
		Base:           t.GenericTextStyle.GetForeground(),
		Title:          t.LogoStyle.GetBackground(),
		Codeblock:      c(charmtone.Salt, lipgloss.Color("#2F2E36")),
		Program:        t.SelectedStyle.GetForeground(),
		Command:        t.SelectedStyle.GetForeground(),
		DimmedArgument: t.SubtleStyle.GetForeground(),
		Comment:        t.SubtleStyle.GetForeground(),
		Flag:           t.SelectedStyle.GetForeground(),
		Argument:       t.GenericTextStyle.GetForeground(),
		Description:    t.GenericTextStyle.GetForeground(),
		FlagDefault:    t.SelectedSubtleStyle.GetForeground(),
		QuotedString:   t.GenericTextStyle.GetForeground(),
		ErrorHeader: [2]color.Color{
			t.ErrorTitleStyle.GetForeground(),
			t.ErrorTitleStyle.GetBackground(),
		},
	}
}
