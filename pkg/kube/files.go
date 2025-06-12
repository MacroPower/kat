package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var (
	// ErrNoCommandForPath is returned when no command is found for a path.
	ErrNoCommandForPath = errors.New("no command for path")

	// ErrCommandExecution is returned when command execution fails.
	ErrCommandExecution = errors.New("command execution")

	// ErrHookExecution is returned when hook execution fails.
	ErrHookExecution = errors.New("hook execution")
)

// HookCommand represents a single hook command to execute.
type HookCommand struct {
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args"    yaml:"args"`
}

func NewHookCommand(command string, args ...string) *HookCommand {
	return &HookCommand{
		Command: command,
		Args:    args,
	}
}

func (hc *HookCommand) Exec(dir string, stdin []byte) error {
	if hc.Command == "" {
		return fmt.Errorf("%w: empty hook command", ErrHookExecution)
	}

	cmd := exec.Command(hc.Command, hc.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// If stdin is provided, pass it to the hook command.
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	if err := cmd.Run(); err != nil {
		var errMsg string
		if stderr.Len() > 0 {
			errMsg += stderr.String() + "\n"
		}
		if stdout.Len() > 0 {
			errMsg += stdout.String() + "\n"
		}

		return fmt.Errorf("%s%w: %w", errMsg, ErrHookExecution, err)
	}

	return nil
}

// Hooks represents the different types of hooks that can be executed.
type Hooks struct {
	PreRender  []*HookCommand `json:"preRender,omitempty"  yaml:"preRender,omitempty"`
	PostRender []*HookCommand `json:"postRender,omitempty" yaml:"postRender,omitempty"`
}

func NewHooks(opts ...HookOpts) *Hooks {
	h := &Hooks{}
	for _, opt := range opts {
		opt(h)
	}

	return h
}

type HookOpts func(*Hooks)

func WithPreRender(hooks ...*HookCommand) HookOpts {
	return func(h *Hooks) {
		h.PreRender = append(h.PreRender, hooks...)
	}
}

func WithPostRender(hooks ...*HookCommand) HookOpts {
	return func(h *Hooks) {
		h.PostRender = append(h.PostRender, hooks...)
	}
}

type Command struct {
	Match   *regexp.Regexp `json:"match"           yaml:"match"` // Regex to match file paths.
	Hooks   *Hooks         `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	Command string         `json:"command"         yaml:"command"`
	Args    []string       `json:"args"            yaml:"args"`
}

func NewCommand(hooks *Hooks, match, cmd string, args ...string) (*Command, error) {
	re, err := regexp.Compile(match)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}

	return &Command{
		Match:   re,
		Command: cmd,
		Args:    args,
		Hooks:   hooks,
	}, nil
}

func MustNewCommand(hooks *Hooks, match, cmd string, args ...string) *Command {
	c, err := NewCommand(hooks, match, cmd, args...)
	if err != nil {
		panic(err)
	}

	return c
}

// Exec runs the command with the given arguments in the specified directory.
func (c *Command) Exec(dir string) (CommandOutput, error) {
	if c.Command == "" {
		return CommandOutput{}, fmt.Errorf("%w: empty command", ErrCommandExecution)
	}

	// Execute preRender hooks, if any.
	if c.Hooks != nil {
		for _, hook := range c.Hooks.PreRender {
			if err := hook.Exec(dir, nil); err != nil {
				return CommandOutput{}, fmt.Errorf("%w: %w", ErrHookExecution, err)
			}
		}
	}

	// Execute main command.
	cmd := exec.Command(c.Command, c.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
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
			return output, fmt.Errorf("%s\n%w: %w", stderr.String(), ErrCommandExecution, err)
		}

		return output, fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	objects, err := SplitYAML(stdout.Bytes())
	if err != nil {
		return output, err
	}

	output.Resources = objects

	// Execute postRender hooks, passing the main command's output as stdin.
	if c.Hooks != nil {
		for _, hook := range c.Hooks.PostRender {
			if err := hook.Exec(dir, stdout.Bytes()); err != nil {
				return output, err
			}
		}
	}

	slog.Debug("returning objects", slog.Int("count", len(objects)))

	return output, nil
}

// CommandRunner manages file-to-command mappings and executes commands based on
// file paths.
type CommandRunner struct {
	path     string
	command  *Command
	commands []*Command
}

// NewCommandRunner creates a new CommandRunner with default command mappings.
func NewCommandRunner(path string) *CommandRunner {
	return &CommandRunner{path, nil, DefaultConfig.Commands}
}

func (cr *CommandRunner) SetCommand(cmd *Command) {
	cr.command = cmd
}

func (cr *CommandRunner) SetCommands(cmds []*Command) {
	cr.commands = cmds
}

// CommandOutput represents the output of a command execution.
type CommandOutput struct {
	Stdout    string
	Stderr    string
	Resources []*Resource
	Hooks     []*HookCommand
}

func (cr *CommandRunner) String() string {
	if cr.command != nil {
		return cr.command.Command
	}

	return "auto"
}

// RunFirstMatch executes the first matching command for the given path.
// If path is a file, it checks for direct matches.
// If path is a directory, it checks all files in the directory for matches.
func (cr *CommandRunner) Run() (CommandOutput, error) {
	slog.Debug("run command", slog.Any("cmd", *cr))

	if cr.command != nil {
		// Custom command provided.
		return cr.command.Exec(cr.path)
	}

	fileInfo, err := os.Stat(cr.path)
	if err != nil {
		return CommandOutput{}, fmt.Errorf("stat path: %w", err)
	}

	if fileInfo.IsDir() {
		// Path is a directory, find matching files inside.
		cmd, err := cr.findMatchInDirectory(cr.path)
		if err != nil {
			return CommandOutput{}, err
		}

		return cmd.Exec(cr.path)
	}

	// Path is a file, check for direct match.
	for _, cmd := range cr.commands {
		if cmd.Match.MatchString(cr.path) {
			return cmd.Exec(filepath.Dir(cr.path))
		}
	}

	return CommandOutput{}, fmt.Errorf("%w: %s", ErrNoCommandForPath, cr.path)
}

// findMatchInDirectory looks for matching files in a directory.
func (cr *CommandRunner) findMatchInDirectory(dirPath string) (*Command, error) {
	var matchedCommand *Command
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dirPath {
			return filepath.SkipDir // Skip subdirectories.
		}
		if !d.IsDir() {
			for _, cmd := range cr.commands {
				if cmd.Match.MatchString(path) {
					matchedCommand = cmd

					return nil
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}
	if matchedCommand == nil {
		return nil, fmt.Errorf("%w: no matching files in %s", ErrNoCommandForPath, dirPath)
	}

	return matchedCommand, nil
}

type ResourceGetter struct {
	Resources []*Resource
}

func NewResourceGetter(input string) (*ResourceGetter, error) {
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	resources, err := SplitYAML([]byte(input))
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	return &ResourceGetter{Resources: resources}, nil
}

func (rg *ResourceGetter) String() string {
	return "static"
}

func (rg *ResourceGetter) Run() (CommandOutput, error) {
	if rg.Resources == nil {
		return CommandOutput{}, errors.New("no resources available")
	}

	return CommandOutput{Resources: rg.Resources}, nil
}
