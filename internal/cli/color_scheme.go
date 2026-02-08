package cli

import (
	"go.jacobcolvin.com/niceyaml/style"

	"github.com/macropower/kat/api/v1beta1/configs"
	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/ui/theme"
)

// LoadStyles returns the [style.Styles] from the user's config, falling back
// to the default theme.
func LoadStyles() style.Styles {
	configPath := configs.GetPath()

	cl, err := config.NewLoaderFromFile(
		configPath,
		configs.New,
		config.WithThemeFromData(),
	)
	if err != nil {
		return theme.Default.Styles
	}

	return cl.GetTheme().Styles
}
