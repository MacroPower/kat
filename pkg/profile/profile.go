// Package profile provides command profile functionality for executing
// commands with optional source filtering using CEL expressions.
//
// Profiles define how to execute commands against sets of files,
// with support for hooks (pre/post execution commands) and source
// filtering to determine which files should be processed.
package profile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"

	"github.com/MacroPower/kat/pkg/expr"
	"github.com/MacroPower/kat/pkg/keys"
)

var (
	// ErrNoCommandForPath is returned when no command is found for a path.
	ErrNoCommandForPath = errors.New("no command for path")

	// ErrCommandExecution is returned when command execution fails.
	ErrCommandExecution = errors.New("command execution")

	// ErrEmptyCommand is returned when a command is empty.
	ErrEmptyCommand = errors.New("empty command")

	// ErrHookExecution is returned when hook execution fails.
	ErrHookExecution = errors.New("hook execution")
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
	sourceProgram cel.Program // Compiled CEL program for source matching.

	Hooks   *Hooks             `yaml:"hooks,omitempty"`
	UI      *UIConfig          `yaml:"ui,omitempty"` // UI configuration for the profile.
	Plugins map[string]*Plugin `yaml:"plugins,omitempty"`
	Source  string             `yaml:"source,omitempty"`
	Command string             `validate:"required,alphanum" yaml:"command"`
	Args    []string           `yaml:"args,flow"`
}

// ProfileOpt is a functional option for configuring a Profile.
type ProfileOpt func(*Profile)

// New creates a new profile with the given command and options.
func New(command string, opts ...ProfileOpt) (*Profile, error) {
	p := &Profile{
		Command: command,
	}
	for _, opt := range opts {
		opt(p)
	}
	if err := p.CompileSource(); err != nil {
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
		p.Args = args
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

// WithPlugins sets the plugins for the profile.
func WithPlugins(plugins map[string]*Plugin) ProfileOpt {
	return func(p *Profile) {
		p.Plugins = plugins
	}
}

// CompileSource compiles the profile's source expression into a CEL program.
func (p *Profile) CompileSource() error {
	if p.sourceProgram == nil && p.Source != "" {
		env, err := expr.CreateEnvironment()
		if err != nil {
			return fmt.Errorf("create CEL environment: %w", err)
		}

		ast, issues := env.Compile(p.Source)
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("compile source expression: %w", issues.Err())
		}

		program, err := env.Program(ast)
		if err != nil {
			return fmt.Errorf("create CEL program: %w", err)
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

// ExecResult represents the result of executing a profile.
type ExecResult struct {
	Error  error
	Stdout string
	Stderr string
}

// Exec runs the profile in the specified directory.
// Returns ExecResult with the command output and any post-render hooks.
func (p *Profile) Exec(ctx context.Context, dir string) ExecResult {
	if p.Command == "" {
		return ExecResult{Error: fmt.Errorf("%w: %w", ErrCommandExecution, ErrEmptyCommand)}
	}

	// Execute preRender hooks, if any.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PreRender {
			if err := hook.Exec(ctx, dir, nil); err != nil {
				return ExecResult{Error: fmt.Errorf("%w: %w", ErrHookExecution, err)}
			}
		}
	}

	// Execute main command.
	cmd := exec.CommandContext(ctx, p.Command, p.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd.Dir = dir

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

	// Execute postRender hooks, passing the main command's output as stdin.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PostRender {
			if err := hook.Exec(ctx, dir, stdout.Bytes()); err != nil {
				result.Error = err

				return result
			}
		}
	}

	slog.DebugContext(ctx, "profile executed successfully", slog.String("command", p.Command))

	return result
}

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

func (p *Profile) GetPluginKeyBinds() []*keys.KeyBind {
	binds := []*keys.KeyBind{}

	if p.Plugins == nil {
		return binds
	}

	for name, plugin := range p.Plugins {
		desc := plugin.Description
		if desc == "" {
			desc = fmt.Sprintf("plugin %q", name)
		}
		binds = append(binds, &keys.KeyBind{
			Description: desc,
			Keys:        plugin.Keys,
		})
	}

	return binds
}
