package profile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Hooks represents the different types of hooks that can be executed.
type Hooks struct {
	Init       []*HookCommand `yaml:"init,omitempty"`
	PreRender  []*HookCommand `yaml:"preRender,omitempty"`
	PostRender []*HookCommand `yaml:"postRender,omitempty"`
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
	Environment Environment `yaml:",inline"`
	Command     string      `validate:"required,alphanum" yaml:"command"`
	Args        []string    `yaml:"args,flow"`
}

// NewHookCommand creates a new hook command with the given command and options.
func NewHookCommand(command string, opts ...HookCommandOpt) (*HookCommand, error) {
	hc := &HookCommand{
		Command: command,
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
		hc.Args = args
	}
}

// WithHookEnvVar sets a single environment variable for the hook command.
func WithHookEnvVar(envVar EnvVar) HookCommandOpt {
	return func(hc *HookCommand) {
		hc.Environment.AddEnvVar(envVar)
	}
}

// WithHookEnvFrom sets the envFrom sources for the hook command.
func WithHookEnvFrom(envFrom []EnvFromSource) HookCommandOpt {
	return func(hc *HookCommand) {
		hc.Environment.AddEnvFrom(envFrom)
	}
}

func (hc *HookCommand) Build() error {
	hc.Environment.SetBaseEnv(os.Environ())

	if err := hc.Environment.CompilePatterns(); err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	return nil
}

// Exec executes the hook command in the given directory with optional stdin.
func (hc *HookCommand) Exec(ctx context.Context, dir string, stdin []byte) error {
	if hc.Command == "" {
		return ErrEmptyCommand
	}

	// Build environment variables for command execution.
	env := hc.Environment.Build()

	cmd := exec.CommandContext(ctx, hc.Command, hc.Args...) //nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd.Dir = dir
	cmd.Env = env

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
