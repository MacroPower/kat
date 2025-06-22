package profile

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Hooks represents the different types of hooks that can be executed.
type Hooks struct {
	Init       []*HookCommand `yaml:"init,omitempty"`
	PreRender  []*HookCommand `yaml:"preRender,omitempty"`
	PostRender []*HookCommand `yaml:"postRender,omitempty"`
}

// NewHooks creates a new Hooks instance with the given options.
func NewHooks(opts ...HookOpts) *Hooks {
	h := &Hooks{}
	for _, opt := range opts {
		opt(h)
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

// HookCommand represents a single hook command to execute.
type HookCommand struct {
	Command string   `validate:"required,alphanum" yaml:"command"`
	Args    []string `yaml:"args,flow"`
}

// NewHookCommand creates a new hook command with the given command and arguments.
func NewHookCommand(command string, args ...string) *HookCommand {
	return &HookCommand{
		Command: command,
		Args:    args,
	}
}

// Exec executes the hook command in the given directory with optional stdin.
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
