package yaml_test

import (
	"errors"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/yaml"
)

func TestYAMLError(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	err := yaml.NewError(
		errors.New("test error"),
		yaml.WithPath(yaml.NewPathBuilder().Root().Child("key").Build()),
		yaml.WithSourceLines(2),
		yaml.WithTheme(theme.New("onedark")),
		yaml.WithFormatter("terminal16m"),
		yaml.WithSource([]byte(`a: b
b: c
foo: "bar"
key: value
baz: 5
c: d
e: f`)),
	)

	require.Error(t, err)
}
