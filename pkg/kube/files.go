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
)

type Command struct {
	Match   *regexp.Regexp
	Command string
	Args    []string
}

func NewCommand(match, cmd string, args ...string) (*Command, error) {
	re, err := regexp.Compile(match)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}

	return &Command{
		Match:   re,
		Command: cmd,
		Args:    args,
	}, nil
}

func MustNewCommand(match, cmd string, args ...string) *Command {
	c, err := NewCommand(match, cmd, args...)
	if err != nil {
		panic(err)
	}

	return c
}

// Exec runs the command with the given arguments in the specified directory.
func (c *Command) Exec(dir string) (CommandOutput, error) {
	slog.Debug("exec", slog.Any("cmd", *c))

	if c.Command == "" {
		return CommandOutput{}, fmt.Errorf("%w: empty command", ErrCommandExecution)
	}

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

	if err != nil {
		if stderr.Len() > 0 {
			return output, fmt.Errorf("%w: %s: %w", ErrCommandExecution, stderr.String(), err)
		}

		return output, fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	objects, err := SplitYAML(stdout.Bytes())
	if err != nil {
		return output, err
	}

	output.Resources = objects

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
