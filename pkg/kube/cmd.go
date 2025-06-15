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

type Command struct {
	Match   *regexp.Regexp `json:"match"            yaml:"match"` // Regex to match file paths.
	Source  *regexp.Regexp `json:"source,omitempty" yaml:"source,omitempty"`
	Hooks   *Hooks         `json:"hooks,omitempty"  yaml:"hooks,omitempty"`
	Command string         `json:"command"          yaml:"command"`
	Args    []string       `json:"args"             yaml:"args"`
}

func NewCommand(hooks *Hooks, match, source, cmd string, args ...string) (*Command, error) {
	c := &Command{
		Command: cmd,
		Args:    args,
		Hooks:   hooks,
	}

	rem, err := regexp.Compile(match)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}
	c.Match = rem

	if source != "" {
		res, err := regexp.Compile(source)
		if err != nil {
			return nil, fmt.Errorf("compile regex: %w", err)
		}
		c.Source = res
	}

	return c, nil
}

func MustNewCommand(hooks *Hooks, match, source, cmd string, args ...string) *Command {
	c, err := NewCommand(hooks, match, source, cmd, args...)
	if err != nil {
		panic(err)
	}

	return c
}

// Exec runs the command with the given arguments in the specified directory.
func (c *Command) Exec(ctx context.Context, dir string) CommandOutput {
	if c.Command == "" {
		return CommandOutput{Error: fmt.Errorf("%w: %w", ErrCommandExecution, ErrEmptyCommand)}
	}

	// Execute preRender hooks, if any.
	if c.Hooks != nil {
		for _, hook := range c.Hooks.PreRender {
			if err := hook.Exec(ctx, dir, nil); err != nil {
				return CommandOutput{Error: fmt.Errorf("%w: %w", ErrHookExecution, err)}
			}
		}
	}

	// Execute main command.
	cmd := exec.CommandContext(ctx, c.Command, c.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := CommandOutput{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if c.Hooks != nil {
		output.Hooks = c.Hooks.PostRender
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
	if c.Hooks != nil {
		for _, hook := range c.Hooks.PostRender {
			if err := hook.Exec(ctx, dir, stdout.Bytes()); err != nil {
				output.Error = err

				return output
			}
		}
	}

	slog.DebugContext(ctx, "returning objects", slog.Int("count", len(objects)))

	return output
}
