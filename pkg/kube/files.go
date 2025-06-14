package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/fsnotify/fsnotify"
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

func (hc *HookCommand) Exec(ctx context.Context, dir string, stdin []byte) error {
	if hc.Command == "" {
		return ErrEmptyCommand
	}

	cmd := exec.CommandContext(ctx, hc.Command, hc.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
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

		if errMsg != "" {
			return fmt.Errorf("%s: %w:\n%s", hc.Command, err, errMsg)
		}

		return fmt.Errorf("%s: %w", hc.Command, err)
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

// CommandEvent represents an event related to command execution.
type CommandEvent any

type (
	// CommandEventStart indicates that a command execution has started.
	CommandEventStart struct{}

	// CommandEventEnd indicates that a command execution has ended.
	// This event carries the output of the command execution, which could be
	// an error if the command failed.
	CommandEventEnd CommandOutput

	// CommandEventCancel indicates that a command execution has been canceled.
	CommandEventCancel struct{}
)

// CommandRunner wraps one or more [Command] objects. It manages:
//   - File-to-command mappings.
//   - Filesystem notifications / watching.
//   - Concurrent command execution.
type CommandRunner struct {
	command    *Command
	watcher    *fsnotify.Watcher
	cancelFunc context.CancelFunc
	path       string
	listeners  []chan<- CommandEvent
	mu         sync.Mutex
}

// NewCommandRunner creates a new [CommandRunner].
func NewCommandRunner(path string, opts ...CommandRunnerOpt) (*CommandRunner, error) {
	cr := &CommandRunner{
		path: path,
	}

	var err error
	cr.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	if len(opts) == 0 {
		// Defaults if no options are provided.
		opts = append(opts, WithCommands(DefaultConfig.Commands))
	}

	for _, opt := range opts {
		if err := opt(cr); err != nil {
			return nil, fmt.Errorf("command option: %w", err)
		}
	}

	if cr.command == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoCommandForPath, cr.path)
	}

	return cr, nil
}

type CommandRunnerOpt func(cr *CommandRunner) error

// WithCommand sets a specific command to run.
func WithCommand(cmd *Command) CommandRunnerOpt {
	return func(cr *CommandRunner) error {
		cr.command = cmd

		return nil
	}
}

// WithCommands sets multiple commands to run.
func WithCommands(cmds []*Command) CommandRunnerOpt {
	return func(cr *CommandRunner) error {
		fileInfo, err := os.Stat(cr.path)
		if err != nil {
			return fmt.Errorf("stat path: %w", err)
		}

		if fileInfo.IsDir() {
			// Path is a directory, find matching files inside.
			cmd, err := findMatchInDirectory(cr.path, cmds)
			if err != nil {
				return err
			}

			cr.command = cmd
		}

		// Path is a file, check for direct match.
		for _, cmd := range cmds {
			if cmd.Match.MatchString(cr.path) {
				cr.command = cmd
			}
		}

		return nil
	}
}

func (cr *CommandRunner) Watch() error {
	err := filepath.Walk(cr.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk %q: %w", path, err)
		}
		if info.IsDir() {
			// Skip directories, we only want to watch files.
			return nil
		}
		if cr.command.Source == nil || cr.command.Source.MatchString(path) {
			// If the file matches the command's regex, add it to the watcher.
			if err := cr.watcher.Add(path); err != nil {
				return fmt.Errorf("add path to watcher: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %q: %w", cr.path, err)
	}

	return nil
}

// Subscribe allows other components to listen for command events.
func (cr *CommandRunner) Subscribe(ch chan<- CommandEvent) {
	cr.listeners = append(cr.listeners, ch)
}

func (cr *CommandRunner) broadcast(evt CommandEvent) {
	// Send the event to all listeners.
	for _, ch := range cr.listeners {
		ch <- evt
	}
}

// RunOnEvent listens for file system events and runs the command in response.
// The output should be collected via [CommandRunner.Subscribe].
func (cr *CommandRunner) RunOnEvent() {
	for {
		select {
		case evt, ok := <-cr.watcher.Events:
			if !ok {
				return
			}
			if evt.Has(fsnotify.Create | fsnotify.Remove | fsnotify.Write | fsnotify.Rename) {
				// Create a new context for this command execution.
				ctx := context.Background()

				// Run the command in a goroutine so we can handle cancellation properly.
				go func() {
					cr.RunContext(ctx)
				}()
			}
		case err, ok := <-cr.watcher.Errors:
			if !ok {
				return
			}
			cr.broadcast(CommandEventEnd(CommandOutput{
				Error: err,
			}))
		}
	}
}

func (cr *CommandRunner) Close() {
	err := cr.watcher.Close()
	if err != nil {
		slog.Error("close watcher", slog.Any("err", err))
	}
}

// CommandOutput represents the output of a command execution.
type CommandOutput struct {
	Error     error
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
func (cr *CommandRunner) Run() CommandOutput {
	return cr.RunContext(context.Background())
}

// RunContext executes the first matching command for the given path with the provided context.
// If path is a file, it checks for direct matches.
// If path is a directory, it checks all files in the directory for matches.
// The context can be used for cancellation, timeouts, and tracing.
func (cr *CommandRunner) RunContext(ctx context.Context) CommandOutput {
	cr.mu.Lock()

	var (
		path = cr.path
		cmd  = cr.command.Command
		args = cr.command.Args
	)

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		cr.broadcast(CommandEventCancel{})
		cr.cancelFunc()
	}

	// Create a new context for this command.
	ctx, cr.cancelFunc = context.WithCancel(ctx)

	cr.mu.Unlock()

	cr.broadcast(CommandEventStart{})

	slog.DebugContext(ctx, "run",
		slog.String("path", path),
		slog.Any("command", cmd),
		slog.Any("args", args),
	)

	_, err := os.Stat(path)
	if err != nil {
		co := CommandOutput{Error: fmt.Errorf("stat path: %w", err)}
		cr.broadcast(CommandEventEnd(co))

		return co
	}

	co := cr.command.Exec(ctx, path)
	cr.broadcast(CommandEventEnd(co))

	return co
}

// findMatchInDirectory looks for matching files in a directory.
func findMatchInDirectory(dirPath string, cmds []*Command) (*Command, error) {
	var matchedCommand *Command
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dirPath {
			return filepath.SkipDir // Skip subdirectories.
		}
		if !d.IsDir() {
			for _, cmd := range cmds {
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
	listeners []chan<- CommandEvent
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

func (rg *ResourceGetter) Run() CommandOutput {
	rg.broadcast(CommandEventStart{})

	out := CommandOutput{Resources: rg.Resources}
	if rg.Resources == nil {
		out.Error = errors.New("no resources available")
	}

	rg.broadcast(CommandEventEnd(out))

	return out
}

func (rg *ResourceGetter) RunOnEvent() {
	// No events to watch for in static resources.
}

func (rg *ResourceGetter) Close() {
	// No resources to close.
}

func (rg *ResourceGetter) Subscribe(ch chan<- CommandEvent) {
	rg.listeners = append(rg.listeners, ch)
}

func (rg *ResourceGetter) broadcast(evt CommandEvent) {
	// Send the event to all listeners.
	for _, ch := range rg.listeners {
		ch <- evt
	}
}
