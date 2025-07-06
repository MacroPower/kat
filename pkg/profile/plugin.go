package profile

import (
	"context"
	"fmt"
	"os"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/keys"
)

// Plugin represents a command plugin that can be executed on demand with keybinds.
type Plugin struct {
	// Command contains the command execution configuration.
	Command execs.Command `json:",inline"`
	// Description provides a description of what the plugin does.
	Description string `json:"description" jsonschema:"title=Description"`
	// Keys defines the key bindings that trigger this plugin.
	Keys []keys.Key `json:"keys,omitempty" jsonschema:"title=Keys"`
}

// NewPlugin creates a new plugin with the given command and options.
func NewPlugin(command, description string, opts ...PluginOpt) (*Plugin, error) {
	p := &Plugin{
		Command: execs.Command{
			Command: command,
		},
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
		p.Command.Args = a
	}
}

// WithPluginKeys sets the keybinds for the plugin.
func WithPluginKeys(k ...keys.Key) PluginOpt {
	return func(p *Plugin) {
		p.Keys = k
	}
}

// WithPluginEnvVar sets a single environment variable for the plugin.
func WithPluginEnvVar(envVar execs.EnvVar) PluginOpt {
	return func(p *Plugin) {
		p.Command.AddEnvVar(envVar)
	}
}

// WithPluginEnvFrom sets the envFrom sources for the plugin.
func WithPluginEnvFrom(envFrom []execs.EnvFromSource) PluginOpt {
	return func(p *Plugin) {
		p.Command.AddEnvFrom(envFrom)
	}
}

func (p *Plugin) Build() error {
	p.Command.SetBaseEnv(os.Environ())

	if err := p.Command.CompilePatterns(); err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	return nil
}

// Exec executes the plugin command in the specified directory.
func (p *Plugin) Exec(ctx context.Context, dir string) (*execs.Result, error) {
	result, err := p.Command.Exec(ctx, dir)
	if err != nil {
		return result, fmt.Errorf("%w: %w", ErrPluginExecution, err)
	}

	return result, nil
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
