package kube

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

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
	rule       *Rule
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
		opts = append(opts, WithRules(DefaultConfig.Rules))
	}

	for _, opt := range opts {
		if err := opt(cr); err != nil {
			return nil, fmt.Errorf("command option: %w", err)
		}
	}

	if cr.rule == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoCommandForPath, cr.path)
	}

	profile := cr.rule.GetProfile()
	if profile.Hooks != nil {
		for _, hook := range profile.Hooks.Init {
			if err := hook.Exec(context.Background(), cr.path, nil); err != nil {
				return nil, fmt.Errorf("%w: init: %w", ErrHookExecution, err)
			}
		}
	}

	return cr, nil
}

type CommandRunnerOpt func(cr *CommandRunner) error

// WithProfile sets a specific profile to use.
func WithProfile(name string, p *Profile) CommandRunnerOpt {
	return func(cr *CommandRunner) error {
		cr.rule = &Rule{
			Profile: name,
			Match:   ".*",
		}
		cr.rule.SetProfile(p)
		if err := cr.rule.CompileMatch(); err != nil {
			return fmt.Errorf("invalid match: %w", err)
		}

		return nil
	}
}

// WithRules sets multiple rules from which the first matching rule will be used.
func WithRules(rules []*Rule) CommandRunnerOpt {
	return func(cr *CommandRunner) error {
		fileInfo, err := os.Stat(cr.path)
		if err != nil {
			return fmt.Errorf("stat path: %w", err)
		}

		if fileInfo.IsDir() {
			// Path is a directory, find matching files inside.
			cmd, err := findMatchInDirectory(cr.path, rules)
			if err != nil {
				return err
			}

			cr.rule = cmd

			return nil
		}

		// Path is a file, check for direct match.
		for _, cmd := range rules {
			if cmd.MatchPath(cr.path) {
				cr.rule = cmd

				return nil
			}
		}

		return nil
	}
}

func (cr *CommandRunner) GetCurrentTheme() string {
	return cr.rule.GetProfile().Theme
}

func (cr *CommandRunner) Watch() error {
	profile := cr.rule.GetProfile()

	err := filepath.Walk(cr.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk %q: %w", path, err)
		}
		if info.IsDir() {
			// Skip directories, we only want to watch files.
			return nil
		}
		if profile.Match(path) {
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
	return cr.rule.String()
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
		path    = cr.path
		profile = cr.rule.GetProfile()
		cmd     = profile.Command
		args    = profile.Args
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

	co := profile.Exec(ctx, path)

	// Check if the command was canceled.
	if co.Error != nil && errors.Is(ctx.Err(), context.Canceled) {
		cr.broadcast(CommandEventCancel{})
	} else {
		cr.broadcast(CommandEventEnd(co))
	}

	return co
}

// findMatchInDirectory looks for matching files in a directory.
func findMatchInDirectory(dirPath string, rules []*Rule) (*Rule, error) {
	var matchedRule *Rule
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dirPath {
			return filepath.SkipDir // Skip subdirectories.
		}
		if !d.IsDir() {
			for _, r := range rules {
				if r.MatchPath(path) {
					matchedRule = r

					return filepath.SkipAll // Stop the walk after finding the first match.
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}
	if matchedRule == nil {
		return nil, fmt.Errorf("%w: no matching files in %s", ErrNoCommandForPath, dirPath)
	}

	return matchedRule, nil
}
