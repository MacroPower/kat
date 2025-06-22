package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/profile"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		opts    []profile.ProfileOpt
		wantErr bool
	}{
		{
			name:    "valid profile",
			command: "echo",
			opts:    []profile.ProfileOpt{profile.WithArgs("hello")},
			wantErr: false,
		},
		{
			name:    "profile with source expression",
			command: "kubectl",
			opts: []profile.ProfileOpt{
				profile.WithArgs("apply", "-f", "-"),
				profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			},
			wantErr: false,
		},
		{
			name:    "profile with invalid source expression",
			command: "kubectl",
			opts: []profile.ProfileOpt{
				profile.WithSource("invalid.expression()"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := profile.New(tt.command, tt.opts...)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, p)
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, tt.command, p.Command)
			}
		})
	}
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	t.Run("valid profile", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("echo", profile.WithArgs("test"))
		require.NotNil(t, p)
		assert.Equal(t, "echo", p.Command)
		assert.Equal(t, []string{"test"}, p.Args)
	})

	t.Run("invalid profile panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			profile.MustNew("test", profile.WithSource("invalid.expression()"))
		})
	})
}

func TestProfile_MatchFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		source        string
		files         []string
		expectedFiles []string
		expectedMatch bool
	}{
		{
			name:          "no source expression",
			source:        "",
			files:         []string{"/app/test.yaml", "/app/config.json"},
			expectedMatch: true,
			expectedFiles: nil, // nil means use default filtering
		},
		{
			name:          "filter yaml files",
			source:        `files.filter(f, pathExt(f) in [".yaml", ".yml"])`,
			files:         []string{"/app/test.yaml", "/app/config.json", "/app/service.yml"},
			expectedMatch: true,
			expectedFiles: []string{"/app/test.yaml", "/app/service.yml"},
		},
		{
			name:          "no matches",
			source:        `files.filter(f, pathExt(f) == ".xml")`,
			files:         []string{"/app/test.yaml", "/app/config.json"},
			expectedMatch: false,
			expectedFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []profile.ProfileOpt{}
			if tt.source != "" {
				opts = append(opts, profile.WithSource(tt.source))
			}

			p, err := profile.New("test", opts...)
			require.NoError(t, err)

			match, files := p.MatchFiles("/app", tt.files)
			assert.Equal(t, tt.expectedMatch, match)
			if tt.expectedFiles != nil {
				assert.ElementsMatch(t, tt.expectedFiles, files)
			} else {
				assert.Nil(t, files)
			}
		})
	}
}

func TestProfile_Exec(t *testing.T) {
	t.Parallel()

	t.Run("successful execution", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("echo", profile.WithArgs("hello", "world"))
		result := p.Exec(t.Context(), "/tmp")

		require.NoError(t, result.Error)
		assert.Contains(t, result.Stdout, "hello world")
		assert.Empty(t, result.Stderr)
	})

	t.Run("failed execution", func(t *testing.T) {
		t.Parallel()

		p := profile.MustNew("false") // command that always fails
		result := p.Exec(t.Context(), "/tmp")

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "command execution")
	})

	t.Run("empty command", func(t *testing.T) {
		t.Parallel()

		p := &profile.Profile{} // empty command
		result := p.Exec(t.Context(), "/tmp")

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "empty command")
	})
}

func TestProfile_WithHooks(t *testing.T) {
	t.Parallel()

	t.Run("successful pre-render hook", func(t *testing.T) {
		t.Parallel()

		hooks := profile.NewHooks(
			profile.WithPreRender(
				profile.NewHookCommand("echo", "pre-render"),
			),
		)

		p := profile.MustNew("echo",
			profile.WithArgs("main command"),
			profile.WithHooks(hooks),
		)

		result := p.Exec(t.Context(), "/tmp")
		require.NoError(t, result.Error)
		assert.Contains(t, result.Stdout, "main command")
	})

	t.Run("failing pre-render hook", func(t *testing.T) {
		t.Parallel()

		hooks := profile.NewHooks(
			profile.WithPreRender(
				profile.NewHookCommand("false"), // always fails
			),
		)

		p := profile.MustNew("echo",
			profile.WithArgs("should not execute"),
			profile.WithHooks(hooks),
		)

		result := p.Exec(t.Context(), "/tmp")
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "hook execution")
		assert.Empty(t, result.Stdout) // main command should not have executed
	})

	t.Run("successful post-render hook", func(t *testing.T) {
		t.Parallel()

		hooks := profile.NewHooks(
			profile.WithPostRender(
				profile.NewHookCommand("grep", "main"),
			),
		)

		p := profile.MustNew("echo",
			profile.WithArgs("main command output"),
			profile.WithHooks(hooks),
		)

		result := p.Exec(t.Context(), "/tmp")
		require.NoError(t, result.Error)
		assert.Contains(t, result.Stdout, "main command output")
	})
}
