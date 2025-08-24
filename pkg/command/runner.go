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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

// ErrNoCommandForPath is returned when no command is found for a path.
var ErrNoCommandForPath = errors.New("no command for path")

// Runner wraps one or more Rule objects. It manages:
//   - File-to-command mappings.
//   - Filesystem notifications / watching.
//   - Concurrent command execution.
type Runner struct {
	tracer   trace.Tracer
	profiles map[string]*profile.Profile
	watcher  *fsnotify.Watcher

	// The root filesystem to operate on. This prevents later re-configuration
	// from escaping the originally configured root path.
	root RootFS

	// Track watched files.
	// Enables directory watching to behave similarly to file watching.
	// Note that it stores absolute file paths (not relative to the root).
	watchedFiles map[string]struct{}

	// Track watched directories.
	// Enables un-watching in re-configuration scenarios.
	// Note that it stores absolute directory paths (not relative to the root).
	watchedDirs map[string]struct{}

	currentProfile     *profile.Profile
	cancelFunc         context.CancelFunc
	path               string
	currentProfileName string
	listeners          []chan<- Event
	allRules           []*rule.Rule
	extraArgs          []string
	mu                 sync.Mutex
	watch              bool
}

// NewRunner creates a new [Runner]. It uses the current working directory as
// the filesystem root.
func NewRunner(path string, opts ...RunnerOpt) (*Runner, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get current working directory: %w", err)
	}

	root, err := os.OpenRoot(wd)
	if err != nil {
		return nil, fmt.Errorf("open root directory %q: %w", wd, err)
	}

	return NewRunnerWithRoot(root, path, opts...)
}

// NewRunner creates a new [Runner] using the provided [RootFS].
// This is not a normal opt since it cannot be re-configured after the runner
// has been created.
func NewRunnerWithRoot(root RootFS, path string, opts ...RunnerOpt) (*Runner, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	cr := &Runner{
		watchedDirs:  make(map[string]struct{}),
		watchedFiles: make(map[string]struct{}),
		profiles:     make(map[string]*profile.Profile),
		tracer:       otel.Tracer("command-runner"),
		root:         root,
		watcher:      watcher,
	}

	if len(opts) == 0 {
		// Defaults if no options are provided.
		opts = append(opts,
			WithRules(DefaultConfig.Rules),
			WithProfiles(DefaultConfig.Profiles))
	}

	opts = append(opts, WithPath(path))

	err = cr.Configure(opts...)
	if err != nil {
		return nil, err
	}

	return cr, nil
}

func (cr *Runner) Configure(opts ...RunnerOpt) error {
	return cr.ConfigureContext(context.Background(), opts...)
}

// Configure applies options to an existing runner.
// This allows reconfiguration after creation.
func (cr *Runner) ConfigureContext(ctx context.Context, opts ...RunnerOpt) error {
	ctx, span := cr.tracer.Start(ctx, "configure")
	defer span.End()

	logger := log.WithContext(ctx)

	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.removeWatchers(ctx)

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		// Note: The cancel event is broadcast by the canceled goroutine.
		cr.cancelFunc()
	}

	// Apply options.
	for _, opt := range opts {
		err := opt(cr)
		if err != nil {
			return fmt.Errorf("apply option: %w", err)
		}
	}

	if cr.currentProfileName != "" && cr.currentProfile == nil {
		var ok bool

		cr.currentProfile, ok = cr.profiles[cr.currentProfileName]
		if !ok {
			return fmt.Errorf("unknown profile: %s", cr.currentProfileName)
		}
	}

	// If we have rules but no current profile set, find the matching rule and set the profile.
	if cr.currentProfile == nil && len(cr.allRules) > 0 {
		pName, p, err := cr.FindProfile(cr.path)
		if err != nil {
			return err
		}

		cr.currentProfileName = pName
		cr.currentProfile = p
	}

	if cr.currentProfile == nil {
		return fmt.Errorf("%w: %s", ErrNoCommandForPath, cr.path)
	}

	if len(cr.extraArgs) > 0 {
		err := cr.setExtraArgs()
		if err != nil {
			return err
		}
	}

	if cr.watch {
		err := cr.watchSource(ctx)
		if err != nil {
			return err
		}
	}

	if cr.currentProfile != nil && cr.currentProfile.Hooks != nil {
		for _, hook := range cr.currentProfile.Hooks.Init {
			hr, err := hook.Exec(ctx, cr.path)
			if err != nil && hr != nil {
				return fmt.Errorf("%w: init: %w\n%s\n%s", profile.ErrHookExecution, err, hr.Stdout, hr.Stderr)
			} else if err != nil {
				return fmt.Errorf("%w: init: %w", profile.ErrHookExecution, err)
			}
		}
	}

	cr.broadcast(NewEventConfigure(ctx))
	logger.DebugContext(ctx, "configured runner",
		slog.String("path", cr.path),
		slog.String("profile", cr.currentProfile.String()),
		slog.Bool("watch", cr.watch),
	)

	return nil
}

type RunnerOpt func(cr *Runner) error

// WithPath sets the path for the runner (relative to the initial root).
// If the path tries to escape the root, it returns an error early to avoid
// runtime errors deeper in the stack.
func WithPath(path string) RunnerOpt {
	return func(cr *Runner) error {
		path = filepath.Clean(path)

		_, err := cr.root.Stat(path)
		if err != nil {
			return fmt.Errorf("stat path %q: %w", path, err)
		}

		cr.path = path

		return nil
	}
}

// WithWatch sets the watch flag for the runner.
func WithWatch(watch bool) RunnerOpt {
	return func(cr *Runner) error {
		cr.watch = watch

		return nil
	}
}

// WithProfile sets a specific profile to use.
func WithProfile(name string) RunnerOpt {
	return func(cr *Runner) error {
		cr.currentProfileName = name
		cr.currentProfile = nil

		return nil
	}
}

// WithProfile sets a custom profile to use.
func WithCustomProfile(name string, p *profile.Profile) RunnerOpt {
	return func(cr *Runner) error {
		cr.currentProfile = p
		cr.currentProfileName = name
		cr.profiles[name] = p

		return nil
	}
}

// WithAutoProfile configures the runner to determine the profile via rules.
func WithAutoProfile() RunnerOpt {
	return func(cr *Runner) error {
		cr.currentProfile = nil
		cr.currentProfileName = ""

		return nil
	}
}

// WithExtraArgs sets additional arguments to pass to the command.
// This will override defined ExtraArgs on whatever profile was selected.
func WithExtraArgs(args ...string) RunnerOpt {
	return func(cr *Runner) error {
		cr.extraArgs = args
		return nil
	}
}

// WithRules sets multiple rules from which the first matching rule will be used.
func WithRules(rs []*rule.Rule) RunnerOpt {
	return func(cr *Runner) error {
		// Store all rules for later use.
		cr.allRules = rs

		// Note: We don't select the initial profile here because profiles might not be loaded yet.
		// The initial profile selection will happen in NewRunner after all options are processed.
		return nil
	}
}

// WithProfiles adds additional profiles to the runner's profile map.
// This allows profiles to be available for switching even if they don't have associated rules.
func WithProfiles(profiles map[string]*profile.Profile) RunnerOpt {
	return func(cr *Runner) error {
		cr.profiles = profiles

		return nil
	}
}

type ProfileMatch struct {
	Profile *profile.Profile
	Name    string
}

func (cr *Runner) FindProfile(path string) (string, *profile.Profile, error) {
	matches, err := cr.FindProfiles(path)
	if err != nil {
		return "", nil, err
	}

	if len(matches) == 0 {
		return "", nil, fmt.Errorf("%w: no matching profile found", ErrNoCommandForPath)
	}

	// Return the highest priority match.
	return matches[0].Name, matches[0].Profile, nil
}

// FindProfiles finds matching profiles for the given path using the configured rules.
// The results are returned in order of priority.
func (cr *Runner) FindProfiles(path string) ([]ProfileMatch, error) {
	path = filepath.Clean(path)

	fileInfo, err := cr.root.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	matches := []ProfileMatch{}

	if fileInfo.IsDir() {
		// Path is a directory, find matching files inside.
		cmds, err := cr.findMatchInDirectory(path)
		if err != nil {
			return nil, err
		}

		for _, cmd := range cmds {
			p, exists := cr.profiles[cmd.Profile]
			if !exists {
				return nil, fmt.Errorf("profile %q not found for rule", cmd.Profile)
			}

			matches = append(matches, ProfileMatch{
				Name:    cmd.Profile,
				Profile: p,
			})
		}
		if len(matches) > 0 {
			return matches, nil
		}
	}

	// Path is a file, check for direct match.
	// Normalize to directory mode: pass parent directory and file list.
	fileDir := filepath.Dir(path)
	for _, r := range cr.allRules {
		if !r.MatchFiles(fileDir, []string{path}) {
			continue
		}

		p, exists := cr.profiles[r.Profile]
		if !exists {
			return nil, fmt.Errorf("profile %q not found for rule", r.Profile)
		}

		matches = append(matches, ProfileMatch{
			Name:    r.Profile,
			Profile: p,
		})
	}
	if len(matches) > 0 {
		return matches, nil
	}

	return nil, fmt.Errorf("%w: no matching rule found", ErrNoCommandForPath)
}

// isFileWatched returns true if the file matched the profile's source expression.
func (cr *Runner) isFileWatched(filePath string) bool {
	if _, isWatched := cr.watchedFiles[filePath]; isWatched {
		return true
	}

	return false
}

func (cr *Runner) GetCurrentProfile() (string, *profile.Profile) {
	return cr.currentProfileName, cr.currentProfile
}

func (cr *Runner) GetProfiles() map[string]*profile.Profile {
	return cr.profiles
}

func (cr *Runner) SetProfile(name string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	p, exists := cr.profiles[name]
	if !exists {
		return fmt.Errorf("profile %q not found", name)
	}

	cr.currentProfile = p
	cr.currentProfileName = name

	return nil
}

// FS creates a [FilteredFS] for the runner that hides directories and files
// unless they match at least one of the configured rules.
func (cr *Runner) FS() (*FilteredFS, error) {
	fsys, err := NewFilteredFS(cr.root, cr.allRules...)
	if err != nil {
		return nil, err
	}

	return fsys, nil
}

// RunPlugin executes a plugin by name.
func (cr *Runner) RunPlugin(name string) Output {
	return cr.RunPluginContext(context.Background(), name)
}

func (cr *Runner) setExtraArgs() error {
	_, p := cr.GetCurrentProfile()

	// Create a copy of the profile to avoid mutating shared profiles.
	profileCopy := *p
	profileCopy.ExtraArgs = cr.extraArgs
	err := profileCopy.Build() // Rebuild the profile to apply changes.
	if err != nil {
		return fmt.Errorf("rebuild profile with extra args: %w", err)
	}

	// Update the current profile with the copy.
	cr.currentProfile = &profileCopy

	return nil
}

// RunPluginContext executes a plugin by name with the provided context.
func (cr *Runner) RunPluginContext(ctx context.Context, name string) Output {
	cr.mu.Lock()

	var (
		path = cr.path
		p    = cr.currentProfile
	)

	ctx, span := cr.tracer.Start(ctx, "plugin", trace.WithAttributes(
		attribute.String("name", name),
		attribute.String("path", path),
	))
	defer span.End()

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		// Note: The cancel event is broadcast by the canceled goroutine.
		cr.cancelFunc()
	}

	// Create a new context for this plugin execution.
	ctx, cr.cancelFunc = context.WithCancel(ctx)

	cr.mu.Unlock()

	cr.broadcast(NewEventStart(ctx, TypePlugin))

	co := NewOutput(TypePlugin)

	plugin := p.GetPlugin(name)
	if plugin == nil {
		co.Error = fmt.Errorf("plugin %q: not found", name)
		cr.broadcast(NewEventEnd(ctx, co))

		return co
	}

	_, err := cr.root.Stat(path)
	if err != nil {
		co.Error = fmt.Errorf("stat path: %w", err)
		cr.broadcast(NewEventEnd(ctx, co))

		return co
	}

	result, err := plugin.Exec(ctx, path)
	co.Error = err
	co.Stdout = result.Stdout
	co.Stderr = result.Stderr

	// Check if the command was canceled.
	if co.Error != nil && errors.Is(ctx.Err(), context.Canceled) {
		cr.broadcast(NewEventCancel(ctx))

		return co
	} else if co.Error != nil {
		co.Error = fmt.Errorf("plugin %q: %w", plugin.Description, co.Error)
		cr.broadcast(NewEventEnd(ctx, co))

		return co
	}

	cr.broadcast(NewEventEnd(ctx, co))

	return co
}

func (cr *Runner) watchSource(ctx context.Context) error {
	var (
		files []string
		p     = cr.currentProfile
	)

	err := fs.WalkDir(cr.root.FS(), cr.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip directories, we only want to match against files.
			return nil
		}

		files = append(files, path)

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %q: %w", cr.path, err)
	}

	cr.watchedFiles = make(map[string]struct{})
	if ok, matchedFiles := p.MatchFiles(cr.path, files); ok {
		for _, file := range matchedFiles {
			dir := filepath.Dir(file)

			// Convert relative path to absolute path for fsnotify.
			absDir, err := cr.root.Open(dir)
			if err != nil {
				return fmt.Errorf("open directory %q in root: %w", dir, err)
			}

			absDirPath := absDir.Name()
			absFilePath := filepath.Join(absDirPath, filepath.Base(file))

			err = absDir.Close()
			if err != nil {
				return fmt.Errorf("close directory %q: %w", absDirPath, err)
			}

			err = cr.watcher.Add(absDirPath)
			if err != nil {
				return fmt.Errorf("add path to watcher: %w", err)
			}

			cr.watchedDirs[absDirPath] = struct{}{}
			cr.watchedFiles[absFilePath] = struct{}{}
		}
	}

	log.WithContext(ctx).DebugContext(ctx, "added file watchers",
		slog.String("path", cr.path),
		slog.Int("count", len(cr.watchedDirs)),
	)

	return nil
}

func (cr *Runner) removeWatchers(ctx context.Context) {
	if cr.watcher == nil || len(cr.watchedDirs) == 0 {
		return
	}

	logger := log.WithContext(ctx)

	removedCount := 0
	for dir := range cr.watchedDirs {
		err := cr.watcher.Remove(dir)
		if errors.Is(err, fsnotify.ErrNonExistentWatch) {
			continue
		}
		if err != nil {
			logger.ErrorContext(ctx, "remove path from watcher", slog.Any("err", err))
		}

		removedCount++
	}

	logger.DebugContext(ctx, "removed file watchers",
		slog.String("path", cr.path),
		slog.Int("count", removedCount),
	)

	clear(cr.watchedDirs)
	clear(cr.watchedFiles)
}

// Subscribe allows other components to listen for command events.
func (cr *Runner) Subscribe(ch chan<- Event) {
	cr.listeners = append(cr.listeners, ch)
}

func (cr *Runner) broadcast(evt Event) {
	ctx := evt.GetContext()

	log.WithContext(ctx).DebugContext(ctx, "broadcasting event",
		slog.String("event", fmt.Sprintf("%T", evt)),
	)

	for _, ch := range cr.listeners {
		ch <- evt
	}
}

// SendEvent allows external components to send events to all listeners.
func (cr *Runner) SendEvent(evt Event) {
	cr.broadcast(evt)
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

			if !cr.isFileWatched(evt.Name) {
				continue
			}

			// Ignore events that are not related to file content changes.
			if evt.Has(fsnotify.Chmod) {
				continue
			}

			// Create a new context for this command execution.
			ctx := context.Background()
			logger := log.WithContext(ctx)

			_, p := cr.GetCurrentProfile()
			if p == nil {
				logger.ErrorContext(ctx, "no profile set for command runner, cannot handle event",
					slog.String("event", evt.String()),
				)

				continue
			}

			if p.Reload != "" {
				matched, err := p.MatchFileEvent(evt.Name, evt.Op)
				if err != nil {
					logger.ErrorContext(ctx, "match file event",
						slog.String("event", evt.String()),
						slog.Any("error", err),
					)
					cr.broadcast(NewEventEnd(ctx,
						NewOutput(TypeRun, WithError(fmt.Errorf("match file event: %w", err))),
					))

					continue
				}
				if !matched {
					continue
				}
			}

			// Run the command in a goroutine so we can handle cancellation properly.
			go cr.RunContext(ctx)

		case err, ok := <-cr.watcher.Errors:
			if !ok {
				return
			}

			cr.broadcast(NewEventEnd(
				context.Background(),
				NewOutput(TypeRun, WithError(err)),
			))
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
	if cr.currentProfile != nil {
		return fmt.Sprintf("%s: %s", cr.currentProfileName, cr.currentProfile.String())
	}

	return "no profile"
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
		p    = cr.currentProfile
		cmd  = p.Command.Command
	)

	ctx, span := cr.tracer.Start(ctx, "run", trace.WithAttributes(
		attribute.String("command", cmd),
		attribute.String("path", path),
	))
	defer span.End()

	// Cancel any currently running command.
	if cr.cancelFunc != nil {
		// Note: The cancel event is broadcast by the canceled goroutine.
		cr.cancelFunc()
	}

	// Create a new context for this command.
	ctx, cr.cancelFunc = context.WithCancel(ctx)

	cr.mu.Unlock()

	cr.broadcast(NewEventStart(ctx, TypeRun))

	co := NewOutput(TypeRun)

	_, err := cr.root.Stat(path)
	if err != nil {
		co.Error = fmt.Errorf("stat path: %w", err)
		cr.broadcast(NewEventEnd(ctx, co))

		return co
	}

	result, err := p.Exec(ctx, path)
	co.Error = err
	if result != nil {
		co.Stdout = result.Stdout
		co.Stderr = result.Stderr
	}

	// Check if the command was canceled.
	if co.Error != nil && errors.Is(ctx.Err(), context.Canceled) {
		cr.broadcast(NewEventCancel(ctx))

		return co
	} else if co.Error != nil {
		co.Error = fmt.Errorf("%s: %w", cmd, co.Error)
		cr.broadcast(NewEventEnd(ctx, co))

		return co
	}

	objects, err := kube.SplitYAML([]byte(co.Stdout))
	if err != nil {
		co.Error = fmt.Errorf("%w: %w", err, co.Error)
	}

	co.Resources = objects
	cr.broadcast(NewEventEnd(ctx, co))

	return co
}

// findMatchInDirectory looks for matching files in a directory.
// It collects all files and allows CEL expressions to operate on the entire collection.
// Returns (rule, files) where files contains the specific files to process, or nil to use profile.source.
func (cr *Runner) findMatchInDirectory(dirPath string) ([]*rule.Rule, error) {
	var files []string

	// Collect all files in the directory (non-recursive).
	err := fs.WalkDir(cr.root.FS(), dirPath, func(path string, d fs.DirEntry, err error) error {
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
		return nil, fmt.Errorf("walk %q: %w", dirPath, err)
	}

	// Try each rule with the full file collection.
	matchedRules := []*rule.Rule{}
	for _, r := range cr.allRules {
		if r.MatchFiles(dirPath, files) {
			matchedRules = append(matchedRules, r)
		}
	}
	if len(matchedRules) > 0 {
		return matchedRules, nil
	}

	return nil, fmt.Errorf("%w: no matching files in %s", ErrNoCommandForPath, dirPath)
}
