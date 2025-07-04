package profile

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/macropower/kat/pkg/keys"
)

// Plugin represents a command plugin that can be executed on demand with keybinds.
type Plugin struct {
	Environment Environment `yaml:",inline"`
	Description string      `yaml:"description"`
	Keys        []keys.Key  `yaml:"keys,omitempty"`
	Command     string      `validate:"required,alphanum" yaml:"command"`
	Args        []string    `yaml:"args,flow"`
}

// NewPlugin creates a new plugin with the given command and options.
func NewPlugin(command, description string, opts ...PluginOpt) (*Plugin, error) {
	p := &Plugin{
		Command:     command,
		Description: description,
	}
	for _, opt := range opts {
		opt(p)
	}
	if err := p.Build(); err != nil {
		return nil, fmt.Errorf("plugin %q: %w", command, err)
	}

	return p, nil
}

// MustNewPlugin creates a new plugin and panics if there's an error.
func MustNewPlugin(command, description string, opts ...PluginOpt) *Plugin {
	p, err := NewPlugin(command, description, opts...)
	if err != nil {
		panic(err)
	}

	return p
}

// PluginOpt is a functional option for configuring a Plugin.
type PluginOpt func(*Plugin)

// WithPluginArgs sets the command arguments for the plugin.
func WithPluginArgs(a ...string) PluginOpt {
	return func(p *Plugin) {
		p.Args = a
	}
}

// WithPluginKeys sets the keybinds for the plugin.
func WithPluginKeys(k ...keys.Key) PluginOpt {
	return func(p *Plugin) {
		p.Keys = k
	}
}

// WithPluginEnvVar sets a single environment variable for the plugin.
func WithPluginEnvVar(envVar EnvVar) PluginOpt {
	return func(p *Plugin) {
		p.Environment.AddEnvVar(envVar)
	}
}

// WithPluginEnvFrom sets the envFrom sources for the plugin.
func WithPluginEnvFrom(envFrom []EnvFromSource) PluginOpt {
	return func(p *Plugin) {
		p.Environment.AddEnvFrom(envFrom)
	}
}

func (p *Plugin) Build() error {
	p.Environment.SetBaseEnv(os.Environ())

	if err := p.Environment.CompilePatterns(); err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	return nil
}

// Exec executes the plugin command in the specified directory.
func (p *Plugin) Exec(ctx context.Context, dir string) ExecResult {
	if p.Command == "" {
		return ExecResult{Error: fmt.Errorf("%w: %w", ErrCommandExecution, ErrEmptyCommand)}
	}

	// Build environment variables for command execution.
	env := p.Environment.Build()

	cmd := exec.CommandContext(ctx, p.Command, p.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if stderr.Len() > 0 {
			result.Error = fmt.Errorf("%s\n%w: %w", stderr.String(), ErrCommandExecution, err)

			return result
		}

		result.Error = fmt.Errorf("%w: %w", ErrCommandExecution, err)

		return result
	}

	slog.DebugContext(ctx, "plugin executed successfully", slog.String("command", p.Command))

	return result
}

// MatchKeys checks if any of the plugin's keys match the given key code.
func (p *Plugin) MatchKeys(keyCode string) bool {
	for _, key := range p.Keys {
		if key.Code == keyCode {
			return true
		}
	}

	return false
}
