package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/keys"
	"github.com/MacroPower/kat/pkg/profile"
)

func TestPlugin_Exec(t *testing.T) {
	t.Parallel()

	t.Run("successful plugin execution", func(t *testing.T) {
		t.Parallel()

		plugin := profile.NewPlugin("echo", "test plugin",
			profile.WithPluginArgs("hello", "world"))

		result := plugin.Exec(t.Context(), "/tmp")

		require.NoError(t, result.Error)
		assert.Contains(t, result.Stdout, "hello world")
		assert.Empty(t, result.Stderr)
	})

	t.Run("failed plugin execution", func(t *testing.T) {
		t.Parallel()

		plugin := profile.NewPlugin("false", "failing plugin") // command that always fails

		result := plugin.Exec(t.Context(), "/tmp")

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "command execution")
	})

	t.Run("empty command", func(t *testing.T) {
		t.Parallel()

		plugin := &profile.Plugin{Description: "empty"} // empty command

		result := plugin.Exec(t.Context(), "/tmp")

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "empty command")
	})
}

func TestPlugin_MatchKeys(t *testing.T) {
	t.Parallel()

	plugin := profile.NewPlugin("test", "test plugin",
		profile.WithPluginKeys(
			keys.New("H"),
			keys.New("ctrl+d"),
		))

	tests := []struct {
		name     string
		keyCode  string
		expected bool
	}{
		{
			name:     "matches first key",
			keyCode:  "H",
			expected: true,
		},
		{
			name:     "matches second key",
			keyCode:  "ctrl+d",
			expected: true,
		},
		{
			name:     "does not match different key",
			keyCode:  "X",
			expected: false,
		},
		{
			name:     "empty key code",
			keyCode:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := plugin.MatchKeys(tt.keyCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProfile_GetPlugin(t *testing.T) {
	t.Parallel()

	plugins := map[string]*profile.Plugin{
		"dry-run": profile.NewPlugin("helm", "helm dry run"),
		"lint":    profile.NewPlugin("helm", "helm lint"),
	}

	p := profile.MustNew("helm",
		profile.WithPlugins(plugins))

	t.Run("get existing plugin", func(t *testing.T) {
		t.Parallel()

		plugin := p.GetPlugin("dry-run")
		require.NotNil(t, plugin)
		assert.Equal(t, "helm dry run", plugin.Description)
	})

	t.Run("get non-existent plugin", func(t *testing.T) {
		t.Parallel()

		plugin := p.GetPlugin("non-existent")
		assert.Nil(t, plugin)
	})

	t.Run("profile with no plugins", func(t *testing.T) {
		t.Parallel()

		p2 := profile.MustNew("kubectl")
		plugin := p2.GetPlugin("any")
		assert.Nil(t, plugin)
	})
}

func TestProfile_GetPluginNameByKey(t *testing.T) {
	t.Parallel()

	plugins := map[string]*profile.Plugin{
		"dry-run": profile.NewPlugin("helm", "helm dry run",
			profile.WithPluginKeys(keys.New("H"))),
		"lint": profile.NewPlugin("helm", "helm lint",
			profile.WithPluginKeys(keys.New("L"))),
	}

	p := profile.MustNew("helm",
		profile.WithPlugins(plugins))

	t.Run("get plugin name by matching key", func(t *testing.T) {
		t.Parallel()

		name := p.GetPluginNameByKey("H")
		assert.Equal(t, "dry-run", name)
	})

	t.Run("get plugin name by different key", func(t *testing.T) {
		t.Parallel()

		name := p.GetPluginNameByKey("L")
		assert.Equal(t, "lint", name)
	})

	t.Run("no plugin matches key", func(t *testing.T) {
		t.Parallel()

		name := p.GetPluginNameByKey("X")
		assert.Empty(t, name)
	})

	t.Run("profile with no plugins", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("kubectl")
		name := p.GetPluginNameByKey("H")
		assert.Empty(t, name)
	})
}
