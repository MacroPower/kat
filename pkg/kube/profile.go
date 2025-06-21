package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
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

type Profile struct {
	sourceExp *regexp.Regexp // Compiled regex for source matching.

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
	if p.sourceExp == nil && p.Source != "" {
		re, err := regexp.Compile(p.Source)
		if err != nil {
			return fmt.Errorf("compile source regex: %w", err)
		}
		p.sourceExp = re
	}

	return nil
}

// MatchSource checks if the given source matches the profile's source regex.
func (p *Profile) Match(source string) bool {
	if p.sourceExp == nil {
		return true // If no source regex is defined, match all sources.
	}

	return p.sourceExp.MatchString(source)
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
