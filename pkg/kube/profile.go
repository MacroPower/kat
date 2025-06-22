package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
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

	Hooks   *Hooks   `yaml:"hooks,omitempty"`
	Source  string   `yaml:"source,omitempty"`
	Theme   string   `yaml:"theme,omitempty"`
	Command string   `validate:"required,alphanum" yaml:"command"`
	Args    []string `yaml:"args,flow"`
}

func NewProfile(command string, opts ...ProfileOpt) (*Profile, error) {
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

func MustNewProfile(command string, opts ...ProfileOpt) *Profile {
	p, err := NewProfile(command, opts...)
	if err != nil {
		panic(err)
	}

	return p
}

type ProfileOpt func(*Profile)

func WithArgs(args ...string) ProfileOpt {
	return func(p *Profile) {
		p.Args = args
	}
}

func WithHooks(hooks *Hooks) ProfileOpt {
	return func(p *Profile) {
		p.Hooks = hooks
	}
}

func WithSource(source string) ProfileOpt {
	return func(p *Profile) {
		p.Source = source
	}
}

func (p *Profile) CompileSource() error {
	if p.sourceProgram == nil && p.Source != "" {
		env, err := createCELEnvironment()
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

// Exec runs the profile in the specified directory.
func (p *Profile) Exec(ctx context.Context, dir string) CommandOutput {
	if p.Command == "" {
		return CommandOutput{Error: fmt.Errorf("%w: %w", ErrCommandExecution, ErrEmptyCommand)}
	}

	// Execute preRender hooks, if any.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PreRender {
			if err := hook.Exec(ctx, dir, nil); err != nil {
				return CommandOutput{Error: fmt.Errorf("%w: %w", ErrHookExecution, err)}
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
	output := CommandOutput{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if p.Hooks != nil {
		output.Hooks = p.Hooks.PostRender
	}

	if err != nil {
		if stderr.Len() > 0 {
			output.Error = fmt.Errorf("%s\n%w: %w", stderr.String(), ErrCommandExecution, err)

			return output
		}

		output.Error = fmt.Errorf("%w: %w", ErrCommandExecution, err)

		return output
	}

	objects, err := SplitYAML(stdout.Bytes())
	if err != nil {
		output.Error = err

		return output
	}

	output.Resources = objects

	// Execute postRender hooks, passing the main command's output as stdin.
	if p.Hooks != nil {
		for _, hook := range p.Hooks.PostRender {
			if err := hook.Exec(ctx, dir, stdout.Bytes()); err != nil {
				output.Error = err

				return output
			}
		}
	}

	slog.DebugContext(ctx, "returning objects", slog.Int("count", len(objects)))

	return output
}
