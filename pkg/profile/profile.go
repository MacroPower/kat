package profile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/expr"
	"github.com/macropower/kat/pkg/keys"
)

var (
	// ErrHookExecution is returned when hook execution fails.
	ErrHookExecution = errors.New("hook execution")

	// ErrPluginExecution is returned when plugin execution fails.
	ErrPluginExecution = errors.New("plugin execution")
)

// Profile represents a command profile with optional source filtering.
//
// The Source field contains a CEL expression that determines which files
// should be processed by this profile. The expression has access to:
//   - `files` (list<string>): All file paths in directory
//   - `dir` (string): The directory path being processed
//
// Source CEL expressions must return a list of files:
//   - `files.filter(f, pathExt(f) in [".yaml", ".yml"])` - returns all YAML files
//   - `files.filter(f, pathBase(f) in ["Chart.yaml", "values.yaml"])` - returns Chart and values files
//   - `files.filter(f, pathBase(f) == "Chart.yaml")` - returns files named Chart.yaml
//   - `files.filter(f, !pathBase(f).matches(".*test.*")) - returns non-test files
//   - `files.filter(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2")` - returns charts with apiVersion v2
//   - `files` - unfiltered list means all files should be processed
//   - `[]` - empty list means no files should be processed
//
// If no Source expression is provided, the profile will use default file filtering.
type Profile struct {
	sourceProgram cel.Program
	// Hooks contains lifecycle hooks for the profile.
	Hooks *Hooks `json:"hooks,omitempty" jsonschema:"title=Hooks"`
	// UI contains UI configuration overrides for this profile.
	UI *UIConfig `json:"ui,omitempty" jsonschema:"title=UI Overrides"`
	// Plugins contains a map of plugin names to Plugin configurations.
	Plugins map[string]*Plugin `json:"plugins,omitempty" jsonschema:"title=Plugins"`
	// Source contains the CEL expression source code for the profile.
	Source string `json:"source,omitempty" jsonschema:"title=Source"`
	// Command contains the command execution configuration.
	Command execs.Command `json:",inline"`
}

// ProfileOpt is a functional option for configuring a Profile.
type ProfileOpt func(*Profile)

// New creates a new profile with the given command and options.
func New(command string, opts ...ProfileOpt) (*Profile, error) {
	p := &Profile{Command: execs.Command{Command: command}}
	for _, opt := range opts {
		opt(p)
	}

	err := p.Build()
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", command, err)
	}

	return p, nil
}

// MustNew creates a new profile and panics if there's an error.
func MustNew(command string, opts ...ProfileOpt) *Profile {
	p, err := New(command, opts...)
	if err != nil {
		panic(err)
	}

	return p
}

// WithArgs sets the command arguments for the profile.
func WithArgs(args ...string) ProfileOpt {
	return func(p *Profile) {
		p.Command.Args = args
	}
}

// WithHooks sets the hooks for the profile.
func WithHooks(hooks *Hooks) ProfileOpt {
	return func(p *Profile) {
		p.Hooks = hooks
	}
}

// WithSource sets the source filtering expression for the profile.
func WithSource(source string) ProfileOpt {
	return func(p *Profile) {
		p.Source = source
	}
}

// WithEnvVar sets a single environment variable for the profile.
// Call multiple times to set multiple environment variables.
func WithEnvVar(envVar execs.EnvVar) ProfileOpt {
	return func(p *Profile) {
		p.Command.AddEnvVar(envVar)
	}
}

// WithEnvFrom sets the envFrom sources for the profile.
func WithEnvFrom(envFrom []execs.EnvFromSource) ProfileOpt {
	return func(p *Profile) {
		p.Command.AddEnvFrom(envFrom)
	}
}

// WithPlugins sets the plugins for the profile.
func WithPlugins(plugins map[string]*Plugin) ProfileOpt {
	return func(p *Profile) {
		p.Plugins = plugins
	}
}

func (p *Profile) Build() error {
	p.Command.SetBaseEnv(os.Environ())
	if p.Hooks != nil {
		err := p.Hooks.Build()
		if err != nil {
			return fmt.Errorf("build hooks: %w", err)
		}
	}
	if p.Plugins != nil {
		for _, plugin := range p.Plugins {
			err := plugin.Build()
			if err != nil {
				return fmt.Errorf("build plugin %q: %w", plugin.Command.Command, err)
			}
		}
	}

	err := p.CompileSource()
	if err != nil {
		return fmt.Errorf("compile source: %w", err)
	}

	err = p.Command.CompilePatterns()
	if err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	return nil
}

// CompileSource compiles the profile's source expression into a CEL program.
func (p *Profile) CompileSource() error {
	if p.sourceProgram == nil && p.Source != "" {
		program, err := expr.DefaultEnvironment.Compile(p.Source)
		if err != nil {
			return fmt.Errorf("compile source expression: %w", err)
		}

		p.sourceProgram = program
	}

	return nil
}

// MatchFiles checks if the profile's source expression matches files in a directory.
// The CEL expression must return a list of strings representing files.
// An empty list means no files should be processed with this profile.
//
// Returns (matches, files) where:
// - matches: true if the profile should be used (non-empty file list)
// - files: specific files that were matched.
func (p *Profile) MatchFiles(dirPath string, files []string) (bool, []string) {
	if p.sourceProgram == nil {
		return true, nil // If no source expression is defined, use default file filtering.
	}

	result, _, err := p.sourceProgram.Eval(map[string]any{
		"files": files,
		"dir":   dirPath,
	})
	if err != nil {
		// If evaluation fails, consider it a non-match.
		return false, nil
	}

	// CEL expression must return a list of files.
	if listVal, ok := result.Value().([]ref.Val); ok {
		var matchedFiles []string
		for _, item := range listVal {
			if str, ok := item.Value().(string); ok {
				matchedFiles = append(matchedFiles, str)
			}
		}
		// If we got a non-empty list, return these specific files.
		if len(matchedFiles) > 0 {
			return true, matchedFiles
		}
		// Empty list means no match.
		return false, nil
	}

	// If the result is not a list, it's an error. CEL expressions must return lists.
	return false, nil
}

// Exec runs the profile in the specified directory.
// Returns ExecResult with the command output and any post-render hooks.
func (p *Profile) Exec(ctx context.Context, dir string) (*execs.Result, error) {
	// Execute preRender hooks, if any.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PreRender {
			hr, err := hook.Exec(ctx, dir)
			if err != nil {
				return hr, fmt.Errorf("%w: preRender: %w", ErrHookExecution, err)
			}
		}
	}

	result, err := p.Command.Exec(ctx, dir)
	if err != nil {
		return result, err //nolint:wrapcheck // Primary command does not need additional context.
	}

	// Execute postRender hooks, passing the main command's output as stdin.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PostRender {
			hr, err := hook.ExecWithStdin(ctx, dir, []byte(result.Stdout))
			if err != nil {
				return hr, fmt.Errorf("%w: postRender: %w", ErrHookExecution, err)
			}
		}
	}

	return result, nil
}

// setEnvFrom applies all envFrom sources to the environment map.
// GetPlugin returns the plugin with the given name, or nil if not found.
func (p *Profile) GetPlugin(name string) *Plugin {
	if p.Plugins == nil {
		return nil
	}

	return p.Plugins[name]
}

// GetPluginNameByKey returns the name of the first plugin that matches the given key code.
func (p *Profile) GetPluginNameByKey(keyCode string) string {
	if p.Plugins == nil {
		return ""
	}

	for name, plugin := range p.Plugins {
		if plugin.MatchKeys(keyCode) {
			slog.Debug("matched plugin",
				slog.String("name", name),
				slog.Any("keys", plugin.Keys),
			)

			return name
		}
	}

	return ""
}

func (p *Profile) GetPluginKeyBinds() []keys.KeyBind {
	binds := []keys.KeyBind{}

	if p.Plugins == nil {
		return binds
	}

	for name, plugin := range p.Plugins {
		desc := plugin.Description
		if desc == "" {
			desc = fmt.Sprintf("plugin %q", name)
		}

		binds = append(binds, keys.KeyBind{
			Description: desc,
			Keys:        plugin.Keys,
		})
	}

	return binds
}
