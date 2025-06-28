package command

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

	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
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

// Runner wraps one or more Rule objects. It manages:
//   - File-to-command mappings.
//   - Filesystem notifications / watching.
//   - Concurrent command execution.
type Runner struct {
	rule       *rule.Rule
	watcher    *fsnotify.Watcher
	cancelFunc context.CancelFunc
	path       string
	listeners  []chan<- Event
	mu         sync.Mutex
}

// NewRunner creates a new [Runner].
func NewRunner(path string, opts ...RunnerOpt) (*Runner, error) {
	cr := &Runner{
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

	p := cr.rule.GetProfile()
	if p.Hooks != nil {
		for _, hook := range p.Hooks.Init {
			if err := hook.Exec(context.Background(), cr.path, nil); err != nil {
				return nil, fmt.Errorf("%w: init: %w", ErrHookExecution, err)
			}
		}
	}

	return cr, nil
}

type RunnerOpt func(cr *Runner) error

// WithProfile sets a specific profile to use.
func WithProfile(name string, p *profile.Profile) RunnerOpt {
	return func(cr *Runner) error {
		r, err := rule.New(name, "true") // Always match in CEL.
		if err != nil {
			return fmt.Errorf("invalid match: %w", err)
		}
		r.SetProfile(p)
		cr.rule = r

		return nil
	}
}

// WithRules sets multiple rules from which the first matching rule will be used.
func WithRules(rs []*rule.Rule) RunnerOpt {
	return func(cr *Runner) error {
		fileInfo, err := os.Stat(cr.path)
		if err != nil {
			return fmt.Errorf("stat path: %w", err)
		}

		if fileInfo.IsDir() {
			// Path is a directory, find matching files inside.
			cmd, err := findMatchInDirectory(cr.path, rs)
			if err != nil {
				return err
			}

			cr.rule = cmd

			return nil
		}

		// Path is a file, check for direct match.
		// Normalize to directory mode: pass parent directory and file list.
		fileDir := filepath.Dir(cr.path)
		for _, r := range rs {
			if r.MatchFiles(fileDir, []string{cr.path}) {
				cr.rule = r

				return nil
			}
		}

		return nil
	}
}

func (cr *Runner) GetCurrentProfile() *profile.Profile {
	return cr.rule.GetProfile()
}

// RunPlugin executes a plugin by name.
func (cr *Runner) RunPlugin(name string) Output {
	return cr.RunPluginContext(context.Background(), name)
}

// RunPluginContext executes a plugin by name with the provided context.
func (cr *Runner) RunPluginContext(ctx context.Context, name string) Output {
	cr.mu.Lock()

	var (
		path = cr.path
		p    = cr.rule.GetProfile()
	)

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		cr.broadcast(EventCancel{})
		cr.cancelFunc()
	}

	// Create a new context for this plugin execution.
	ctx, cr.cancelFunc = context.WithCancel(ctx)

	cr.mu.Unlock()

	cr.broadcast(EventStart(TypePlugin))

	slog.DebugContext(ctx, "run plugin",
		slog.String("path", path),
		slog.String("name", name),
	)

	co := Output{
		Type: TypePlugin,
	}

	plugin := p.GetPlugin(name)
	if plugin == nil {
		co.Error = fmt.Errorf("plugin %q not found", name)
		cr.broadcast(EventEnd(co))

		return co
	}

	_, err := os.Stat(path)
	if err != nil {
		co.Error = fmt.Errorf("stat path: %w", err)
		cr.broadcast(EventEnd(co))

		return co
	}

	result := plugin.Exec(ctx, path)
	co.Error = result.Error
	co.Stdout = result.Stdout
	co.Stderr = result.Stderr

	// Check if the command was canceled.
	if co.Error != nil && errors.Is(ctx.Err(), context.Canceled) {
		cr.broadcast(EventCancel{})

		return co
	} else if co.Error != nil {
		co.Error = fmt.Errorf("%w: plugin %q: %w", ErrCommandExecution, plugin.Description, co.Error)
		cr.broadcast(EventEnd(co))

		return co
	}

	cr.broadcast(EventEnd(co))

	return co
}

func (cr *Runner) Watch() error {
	p := cr.rule.GetProfile()

	var files []string
	err := filepath.Walk(cr.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk %q: %w", path, err)
		}
		if info.IsDir() {
			// Skip directories, we only want to watch files.
			return nil
		}
		files = append(files, path)

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %q: %w", cr.path, err)
	}

	if ok, matchedFiles := p.MatchFiles(cr.path, files); ok {
		for _, file := range matchedFiles {
			if err := cr.watcher.Add(file); err != nil {
				return fmt.Errorf("add path to watcher: %w", err)
			}
		}
	}

	return nil
}

// Subscribe allows other components to listen for command events.
func (cr *Runner) Subscribe(ch chan<- Event) {
	cr.listeners = append(cr.listeners, ch)
}

func (cr *Runner) broadcast(evt Event) {
	// Send the event to all listeners.
	for _, ch := range cr.listeners {
		ch <- evt
	}
}

// RunOnEvent listens for file system events and runs the command in response.
// The output should be collected via [Runner.Subscribe].
func (cr *Runner) RunOnEvent() {
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
			cr.broadcast(EventEnd(Output{
				Error: err,
			}))
		}
	}
}

func (cr *Runner) Close() {
	err := cr.watcher.Close()
	if err != nil {
		slog.Error("close watcher", slog.Any("err", err))
	}
}

func (cr *Runner) String() string {
	return cr.rule.String()
}

// RunFirstMatch executes the first matching command for the given path.
// If path is a file, it checks for direct matches.
// If path is a directory, it checks all files in the directory for matches.
func (cr *Runner) Run() Output {
	return cr.RunContext(context.Background())
}

// RunContext executes the first matching command for the given path with the provided context.
// If path is a file, it checks for direct matches.
// If path is a directory, it checks all files in the directory for matches.
// The context can be used for cancellation, timeouts, and tracing.
func (cr *Runner) RunContext(ctx context.Context) Output {
	cr.mu.Lock()

	var (
		path = cr.path
		p    = cr.rule.GetProfile()
		cmd  = p.Command
		args = p.Args
	)

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		cr.broadcast(EventCancel{})
		cr.cancelFunc()
	}

	// Create a new context for this command.
	ctx, cr.cancelFunc = context.WithCancel(ctx)

	cr.mu.Unlock()

	cr.broadcast(EventStart(TypeRun))

	slog.DebugContext(ctx, "run",
		slog.String("path", path),
		slog.Any("command", cmd),
		slog.Any("args", args),
	)

	co := Output{
		Type: TypeRun,
	}

	_, err := os.Stat(path)
	if err != nil {
		co.Error = fmt.Errorf("stat path: %w", err)
		cr.broadcast(EventEnd(co))

		return co
	}

	result := p.Exec(ctx, path)
	co.Error = result.Error
	co.Stdout = result.Stdout
	co.Stderr = result.Stderr

	// Check if the command was canceled.
	if co.Error != nil && errors.Is(ctx.Err(), context.Canceled) {
		cr.broadcast(EventCancel{})

		return co
	} else if co.Error != nil {
		co.Error = fmt.Errorf("%w: %w", ErrCommandExecution, co.Error)
		cr.broadcast(EventEnd(co))

		return co
	}

	objects, err := kube.SplitYAML([]byte(co.Stdout))
	if err != nil {
		co.Error = fmt.Errorf("%w: %w", err, co.Error)
	}
	co.Resources = objects
	cr.broadcast(EventEnd(co))

	return co
}

// findMatchInDirectory looks for matching files in a directory.
// It collects all files and allows CEL expressions to operate on the entire collection.
// Returns (rule, files) where files contains the specific files to process, or nil to use profile.source.
func findMatchInDirectory(dirPath string, rs []*rule.Rule) (*rule.Rule, error) {
	var files []string

	// Collect all files in the directory (non-recursive).
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dirPath {
			return filepath.SkipDir // Skip subdirectories.
		}
		if !d.IsDir() {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	// Try each rule with the full file collection.
	for _, r := range rs {
		if r.MatchFiles(dirPath, files) {
			return r, nil
		}
	}

	return nil, fmt.Errorf("%w: no matching files in %s", ErrNoCommandForPath, dirPath)
}
