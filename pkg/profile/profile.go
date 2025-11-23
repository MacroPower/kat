package profile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/expr"
	"github.com/macropower/kat/pkg/keys"
)

var (
	// ErrHookExecution is returned when hook execution fails.
	ErrHookExecution = errors.New("hook")

	// ErrPluginExecution is returned when plugin execution fails.
	ErrPluginExecution = errors.New("plugin")
)

// Executor executes a profile's commands.
type Executor interface {
	Exec(ctx context.Context, dir string) (*execs.Result, error)
	ExecWithStdin(ctx context.Context, dir string, stdin []byte) (*execs.Result, error)
	String() string
}

// StatusManager manages the status of a profile.
type StatusManager interface {
	SetError(ctx context.Context)
	SetResult(result RenderResult)
	SetStage(stage RenderStage)
	RenderMap() map[string]any
}

// Profile represents a command profile.
type Profile struct {
	sourceProgram *expr.LazyProgram
	reloadProgram *expr.LazyProgram
	executor      Executor
	status        StatusManager

	// Hooks contains lifecycle hooks for the profile.
	Hooks *Hooks `json:"hooks,omitempty" jsonschema:"title=Hooks"`

	// UI contains UI configuration overrides for this profile.
	UI *UIConfig `json:"ui,omitempty" jsonschema:"title=UI Overrides"`

	// Plugins contains a map of plugin names to Plugin configurations.
	Plugins map[string]*Plugin `json:"plugins,omitempty" jsonschema:"title=Plugins"`

	// Source is a CEL expression that determines which files should be watched by
	// this profile, when file watching is enabled. The expression has access to:
	//   - `files` (list<string>): All file paths in directory
	//   - `dir` (string): The directory path being processed
	//
	// Source CEL expressions must return a list of files:
	//   - `files.filter(f, pathExt(f) in [".yaml", ".yml"])` - returns all YAML files
	//   - `files.filter(f, pathBase(f) in ["Chart.yaml", "values.yaml"])` - returns Chart and values files
	//   - `files.filter(f, pathBase(f) == "Chart.yaml")` - returns files named Chart.yaml
	//   - `files.filter(f, !pathBase(f).matches(".*test.*"))` - returns non-test files
	//   - `files.filter(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2")` - returns charts with apiVersion v2
	//   - `files` - unfiltered list means all files should be processed
	//   - `[]` - empty list means no files should be processed
	//
	// If no Source expression is provided, the profile will match all files by default.
	Source string `json:"source,omitempty" jsonschema:"title=Source"`

	// Reload contains a CEL expression that is evaluated on automated reload
	// events (e.g. from watch). If the expression returns true, the reload will proceed.
	// If it returns false, the reload will be skipped. The expression has access to:
	//   - `file` (string): The file path that triggered the event
	//   - `fs.event` (int): The file event type, at least one of `fs.CREATE`, `fs.WRITE`, `fs.REMOVE`, `fs.RENAME`, `fs.CHMOD`
	//   - `render.stage` (int): The current render stage, one of `render.STAGE_NONE`, `render.STAGE_PRE_RENDER`, `render.STAGE_RENDER`, `render.STAGE_POST_RENDER`
	//   - `render.result` (string): The result of the last render operation, one of `render.RESULT_OK`, `render.RESULT_ERROR`, `render.RESULT_CANCEL`
	//
	// Reload CEL expressions must return a boolean value:
	//   - `pathBase(file) != "kustomization.yaml"` - skip reload for kustomization.yaml files
	//   - `fs.event.has(fs.WRITE, fs.RENAME)` - reload for write or rename events
	//   - `render.result != render.RESULT_CANCEL` - skip reload if the last render was canceled
	//   - `render.stage < render.STAGE_RENDER` - only reload if the main render stage has not started
	//
	// If no Reload expression is provided, the profile will always reload on any events.
	Reload string `json:"reload,omitempty" jsonschema:"title=Reload"`

	// Command contains the command execution configuration.
	Command execs.Command `json:",inline"`

	// ExtraArgs contains extra arguments that can be overridden from the CLI.
	// They are appended to the Args of the Command.
	ExtraArgs []string `json:"extraArgs,omitempty" jsonschema:"title=Optional Arguments" yaml:"extraArgs,flow,omitempty"`

	customExecutor bool // TODO: Prevent duplicate builds and remove this.
}

// ProfileOpt is a functional option for configuring a Profile.
type ProfileOpt func(*Profile)

// New creates a new profile with the given command and options.
func New(command string, opts ...ProfileOpt) (*Profile, error) {
	p := &Profile{
		Command: execs.Command{Command: command},
	}
	for _, opt := range opts {
		opt(p)
	}

	err := p.Build()
	if err != nil {
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
		p.Command.Args = args
	}
}

// WithExtraArgs sets extra command arguments for the profile.
// These arguments can be overridden from the CLI.
func WithExtraArgs(args ...string) ProfileOpt {
	return func(p *Profile) {
		p.ExtraArgs = args
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

// WithReload sets the reload expression for the profile.
func WithReload(reload string) ProfileOpt {
	return func(p *Profile) {
		p.Reload = reload
	}
}

// WithEnvVar sets a single environment variable for the profile.
// Call multiple times to set multiple environment variables.
func WithEnvVar(envVar execs.EnvVar) ProfileOpt {
	return func(p *Profile) {
		p.Command.AddEnvVar(envVar)
	}
}

// WithEnvFrom sets the envFrom sources for the profile.
func WithEnvFrom(envFrom []execs.EnvFromSource) ProfileOpt {
	return func(p *Profile) {
		p.Command.AddEnvFrom(envFrom)
	}
}

// WithPlugins sets the plugins for the profile.
func WithPlugins(plugins map[string]*Plugin) ProfileOpt {
	return func(p *Profile) {
		p.Plugins = plugins
	}
}

// WithExecutor sets the [Executor] for the profile.
func WithExecutor(executor Executor) ProfileOpt {
	return func(p *Profile) {
		p.customExecutor = true
		p.executor = executor
	}
}

// WithStatusManager sets the [StatusManager] for the profile.
func WithStatusManager(status StatusManager) ProfileOpt {
	return func(p *Profile) {
		p.status = status
	}
}

func (p *Profile) Build() error {
	if p.status == nil {
		p.status = &Status{}
	}

	p.Command.SetBaseEnv(os.Environ())
	if p.Hooks != nil {
		err := p.Hooks.Build()
		if err != nil {
			return fmt.Errorf("build hooks: %w", err)
		}
	}
	if p.Plugins != nil {
		for _, plugin := range p.Plugins {
			err := plugin.Build()
			if err != nil {
				return fmt.Errorf("build plugin %q: %w", plugin.Command.Command, err)
			}
		}
	}

	err := p.CompileSource()
	if err != nil {
		return fmt.Errorf("compile source: %w", err)
	}

	err = p.CompileReload()
	if err != nil {
		return fmt.Errorf("compile reload: %w", err)
	}

	err = p.Command.CompilePatterns()
	if err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	if p.executor == nil || !p.customExecutor {
		p.executor = execs.NewExecutor(p.Command, p.ExtraArgs...)
	}

	return nil
}

// CompileSource compiles the profile's source expression into a CEL program.
func (p *Profile) CompileSource() error {
	if p.Source == "" {
		return nil
	}

	if p.sourceProgram == nil {
		env, err := expr.NewEnvironment(
			cel.Variable("files", cel.ListType(cel.StringType)),
			cel.Variable("dir", cel.StringType),
		)
		if err != nil {
			return fmt.Errorf("environment: %w", err)
		}

		p.sourceProgram = expr.NewLazyProgram(p.Source, env)
	}

	_, err := p.sourceProgram.Get()
	if err != nil {
		return fmt.Errorf("expression: %w", err)
	}

	return nil
}

// CompileReload compiles the profile's reload expression into a CEL program.
func (p *Profile) CompileReload() error {
	if p.Reload == "" {
		return nil
	}

	if p.reloadProgram == nil {
		env, err := expr.NewEnvironment(
			cel.Variable("file", cel.StringType),
			cel.Variable("fs.event", cel.IntType),
			RenderLib(),
		)
		if err != nil {
			return fmt.Errorf("environment: %w", err)
		}

		p.reloadProgram = expr.NewLazyProgram(p.Reload, env)
	}

	_, err := p.reloadProgram.Get()
	if err != nil {
		return fmt.Errorf("expression: %w", err)
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

	program, err := p.sourceProgram.Get()
	if err != nil {
		// If compilation fails, consider it a non-match.
		return false, nil
	}

	result, _, err := program.Eval(map[string]any{
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

// MatchFileEvent evaluates the profile's reload expression against a file system event.
// Returns true if the reload should proceed, false if it should be skipped.
// If no reload expression is configured, it always returns true.
func (p *Profile) MatchFileEvent(filePath string, fsOp fsnotify.Op) (bool, error) {
	if p.reloadProgram == nil {
		return true, nil // If no reload expression is defined, always reload.
	}

	program, err := p.reloadProgram.Get()
	if err != nil {
		return false, fmt.Errorf("compile reload expression: %w", err)
	}

	evalVars := map[string]any{
		"file":     filePath,
		"fs.event": int64(fsOp),
		"render":   p.status.RenderMap(),
	}

	result, _, err := program.Eval(evalVars)
	if err != nil {
		return false, fmt.Errorf("evaluate reload expression: %w", err)
	}

	resultVal, ok := result.Value().(bool)
	if !ok {
		return false, errors.New("reload expression did not return a boolean value")
	}

	if !resultVal {
		slog.Debug("skipping reload",
			slog.Any("vars", evalVars),
			slog.Bool("result", resultVal),
		)
	}

	return resultVal, nil
}

// Exec runs the profile in the specified directory.
// Returns ExecResult with the command output and any post-render hooks.
func (p *Profile) Exec(ctx context.Context, dir string) (*execs.Result, error) {
	// Execute preRender hooks, if any.
	if p.Hooks != nil {
		p.status.SetStage(StagePreRender)

		for _, hook := range p.Hooks.PreRender {
			hr, err := hook.Exec(ctx, dir)
			if err != nil {
				p.status.SetError(ctx)
				return hr, fmt.Errorf("%w: preRender: %w", ErrHookExecution, err)
			}
		}
	}

	p.status.SetStage(StageRender)

	result, err := p.executor.Exec(ctx, dir)
	if err != nil {
		p.status.SetError(ctx)
		return result, err //nolint:wrapcheck // Primary command does not need additional context.
	}

	// Execute postRender hooks, passing the main command's output as stdin.
	if p.Hooks != nil {
		p.status.SetStage(StagePostRender)

		for _, hook := range p.Hooks.PostRender {
			hr, err := hook.ExecWithStdin(ctx, dir, []byte(result.Stdout))
			if err != nil {
				p.status.SetError(ctx)
				return hr, fmt.Errorf("%w: postRender: %w", ErrHookExecution, err)
			}
		}
	}

	p.status.SetResult(ResultOK)

	return result, nil
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

func (p *Profile) GetPluginKeyBinds() []keys.KeyBind {
	binds := []keys.KeyBind{}

	if p.Plugins == nil {
		return binds
	}

	for name, plugin := range p.Plugins {
		desc := plugin.Description
		if desc == "" {
			desc = fmt.Sprintf("plugin %q", name)
		}

		binds = append(binds, keys.KeyBind{
			Description: desc,
			Keys:        plugin.Keys,
		})
	}

	return binds
}

func (p *Profile) String() string {
	return p.executor.String()
}
