package profile

import (
	"context"
	"fmt"
	"os"

	"github.com/macropower/kat/pkg/execs"
)

// Hooks represents the different types of hooks that can be executed.
type Hooks struct {
	Init       []*HookCommand `json:"init,omitempty"`
	PreRender  []*HookCommand `json:"preRender,omitempty"`
	PostRender []*HookCommand `json:"postRender,omitempty"`
}

// NewHooks creates a new Hooks instance with the given options.
func NewHooks(opts ...HookOpts) (*Hooks, error) {
	h := &Hooks{}
	for _, opt := range opts {
		opt(h)
	}
	if err := h.Build(); err != nil {
		return nil, fmt.Errorf("build hooks: %w", err)
	}

	return h, nil
}

// MustNewHooks creates a new Hooks instance and panics if there's an error.
func MustNewHooks(opts ...HookOpts) *Hooks {
	h, err := NewHooks(opts...)
	if err != nil {
		panic(err)
	}

	return h
}

// HookOpts is a functional option for configuring Hooks.
type HookOpts func(*Hooks)

// WithInit adds init hooks.
func WithInit(hooks ...*HookCommand) HookOpts {
	return func(h *Hooks) {
		h.Init = append(h.Init, hooks...)
	}
}

// WithPreRender adds pre-render hooks.
func WithPreRender(hooks ...*HookCommand) HookOpts {
	return func(h *Hooks) {
		h.PreRender = append(h.PreRender, hooks...)
	}
}

// WithPostRender adds post-render hooks.
func WithPostRender(hooks ...*HookCommand) HookOpts {
	return func(h *Hooks) {
		h.PostRender = append(h.PostRender, hooks...)
	}
}

func (h *Hooks) Build() error {
	for _, cmd := range h.Init {
		err := cmd.Build()
		if err != nil {
			return fmt.Errorf("init hook: %w", err)
		}
	}
	for _, cmd := range h.PreRender {
		err := cmd.Build()
		if err != nil {
			return fmt.Errorf("preRender hook: %w", err)
		}
	}
	for _, cmd := range h.PostRender {
		err := cmd.Build()
		if err != nil {
			return fmt.Errorf("postRender hook: %w", err)
		}
	}

	return nil
}

// HookCommand represents a single hook command to execute.
type HookCommand struct {
	Command execs.Command `json:",inline"`
}

// NewHookCommand creates a new hook command with the given command and options.
func NewHookCommand(command string, opts ...HookCommandOpt) (*HookCommand, error) {
	hc := &HookCommand{
		Command: execs.Command{
			Command: command,
		},
	}
	for _, opt := range opts {
		opt(hc)
	}
	if err := hc.Build(); err != nil {
		return nil, fmt.Errorf("hook %q: %w", command, err)
	}

	return hc, nil
}

// MustNewHookCommand creates a new hook command and panics if there's an error.
func MustNewHookCommand(command string, opts ...HookCommandOpt) *HookCommand {
	hc, err := NewHookCommand(command, opts...)
	if err != nil {
		panic(err)
	}

	return hc
}

// HookCommandOpt is a functional option for configuring a [HookCommand].
type HookCommandOpt func(*HookCommand)

// WithHookArgs sets the command arguments for the hook command.
func WithHookArgs(args ...string) HookCommandOpt {
	return func(hc *HookCommand) {
		hc.Command.Args = args
	}
}

// WithHookEnvVar sets a single environment variable for the hook command.
func WithHookEnvVar(envVar execs.EnvVar) HookCommandOpt {
	return func(hc *HookCommand) {
		hc.Command.AddEnvVar(envVar)
	}
}

// WithHookEnvFrom sets the envFrom sources for the hook command.
func WithHookEnvFrom(envFrom []execs.EnvFromSource) HookCommandOpt {
	return func(hc *HookCommand) {
		hc.Command.AddEnvFrom(envFrom)
	}
}

func (hc *HookCommand) Build() error {
	hc.Command.SetBaseEnv(os.Environ())

	if err := hc.Command.CompilePatterns(); err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	return nil
}

// Exec executes the hook command in the given directory.
func (hc *HookCommand) Exec(ctx context.Context, dir string) (*execs.Result, error) {
	return hc.ExecWithStdin(ctx, dir, nil)
}

func (hc *HookCommand) ExecWithStdin(ctx context.Context, dir string, stdin []byte) (*execs.Result, error) {
	result, err := hc.Command.ExecWithStdin(ctx, dir, stdin)
	if err != nil {
		return result, fmt.Errorf("%w: %w", ErrHookExecution, err)
	}

	return result, nil
}
