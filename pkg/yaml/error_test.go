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

	err := yaml.Error{
		Source: []byte(`a: b
b: c
foo: "bar"
key: value
baz: 5
c: d
e: f`),
		Path:        yaml.NewPathBuilder().Root().Child("key").Build(),
		Err:         errors.New("test error"),
		Theme:       theme.New("onedark"),
		SourceLines: 2,
		Formatter:   "terminal16m",
	}

	require.NoError(t, err)
}
