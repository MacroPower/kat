package execs

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/macropower/kat/pkg/log"
)

type Executor struct {
	tracer    trace.Tracer
	cmd       Command
	extraArgs []string
}

func NewExecutor(cmd Command, args ...string) Executor {
	return Executor{
		tracer:    otel.Tracer("executor"),
		cmd:       cmd,
		extraArgs: args,
	}
}

func (e Executor) Exec(ctx context.Context, dir string) (*Result, error) {
	return e.ExecWithStdin(ctx, dir, nil)
}

func (e Executor) ExecWithStdin(ctx context.Context, dir string, stdin []byte) (*Result, error) {
	ctx, span := e.tracer.Start(ctx, "exec", trace.WithAttributes(
		attribute.String("command", e.String()),
		attribute.String("path", dir),
	))
	defer span.End()

	if e.cmd.Command == "" {
		return nil, ErrEmptyCommand
	}

	logger := log.WithContext(ctx).With(
		slog.String("command", e.String()),
		slog.String("path", dir),
	)

	start := time.Now()

	// Get environment variables for command execution.
	env := e.cmd.GetEnv()

	// Combine Args and ExtraArgs to get the full command arguments.
	allArgs := append([]string{}, e.cmd.Args...)
	allArgs = append(allArgs, e.extraArgs...)

	// Prepare the command to execute.
	//nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd := exec.CommandContext(ctx, e.cmd.Command, allArgs...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		logger.DebugContext(ctx, "command failed",
			slog.Duration("duration", time.Since(start)),
			slog.Any("error", err),
		)

		if stdout.Len() > 0 || stderr.Len() > 0 {
			return result, fmt.Errorf("%w: %w", ErrCommandExecution, err)
		}

		return nil, fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	logger.DebugContext(ctx, "command executed successfully",
		slog.Duration("duration", time.Since(start)),
	)

	return result, nil
}

func (e Executor) String() string {
	allArgs := append([]string{}, e.cmd.Args...)
	allArgs = append(allArgs, e.extraArgs...)

	return fmt.Sprintf("%s %s", e.cmd.Command, strings.Join(allArgs, " "))
}
