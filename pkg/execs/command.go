package execs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

var (
	// ErrCommandExecution is returned when command execution fails.
	ErrCommandExecution = errors.New("run")

	// ErrEmptyCommand is returned when a command is empty.
	ErrEmptyCommand = errors.New("empty command")
)

// Result represents the result of a command execution.
type Result struct {
	Stdout string
	Stderr string
}

// EnvFromSource represents a source for inheriting environment variables.
type EnvFromSource struct {
	// CallerRef specifies how to inherit environment variables from the caller process.
	CallerRef *CallerRef `json:"callerRef,omitempty" jsonschema:"title=Caller Reference"`
}

// CallerRef represents a reference to environment variables from the caller process.
type CallerRef struct {
	compiledPattern *regexp.Regexp // Compiled regex pattern for matching environment variables.

	// Pattern is a regex pattern for matching environment variable names.
	Pattern string `json:"pattern,omitempty" jsonschema:"title=Pattern,format=regex"`
	// Name is the specific environment variable name to inherit.
	Name string `json:"name,omitempty" jsonschema:"title=Name"`
}

// EnvVar represents an environment variable definition.
type EnvVar struct {
	// ValueFrom specifies a source for the environment variable value.
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty" jsonschema:"title=Value From"`
	// Name is the environment variable name.
	Name string `json:"name" jsonschema:"title=Name"`
	// Value is the environment variable value.
	Value string `json:"value,omitempty" jsonschema:"title=Value"`
}

// EnvVarSource represents a source for an environment variable value.
type EnvVarSource struct {
	// CallerRef specifies how to get the value from the caller process environment.
	CallerRef *CallerRef `json:"callerRef,omitempty" jsonschema:"title=Caller Reference"`
}

// Compile compiles the caller reference pattern into a regex if a pattern is provided.
func (c *CallerRef) Compile() error {
	if c.compiledPattern == nil && c.Pattern != "" {
		pattern, err := regexp.Compile(c.Pattern)
		if err != nil {
			return fmt.Errorf("compile pattern %q: %w", c.Pattern, err)
		}

		c.compiledPattern = pattern
	}

	return nil
}

// Command manages common command execution properties.
type Command struct {
	baseEnv map[string]string
	// Command is the command to execute.
	Command string `json:"command" jsonschema:"title=Command,pattern=^\\S+$"`
	// Args contains the command line arguments.
	Args []string `json:"args,omitempty" jsonschema:"title=Arguments" yaml:"args,flow,omitempty"`
	// Env contains environment variable definitions.
	Env []EnvVar `json:"env,omitempty" jsonschema:"title=Environment Variables"`
	// EnvFrom contains sources for inheriting environment variables.
	EnvFrom []EnvFromSource `json:"envFrom,omitempty" jsonschema:"title=Environment Variables From"`
}

// NewCommand creates a new [Command].
// It accepts a base environment, which usually will be from [os.Environ].
func NewCommand(baseEnv []string) Command {
	e := Command{
		Env:     []EnvVar{},
		EnvFrom: []EnvFromSource{},
	}
	e.SetBaseEnv(baseEnv)

	return e
}

func (e *Command) SetBaseEnv(baseEnv []string) {
	// Reset base environment.
	e.baseEnv = make(map[string]string)
	// Parse new base environment into map.
	for _, envVar := range baseEnv {
		if eqIdx := strings.Index(envVar, "="); eqIdx != -1 {
			key := envVar[:eqIdx]
			value := envVar[eqIdx+1:]
			e.baseEnv[key] = value
		}
	}
}

// AddEnvVar adds a single environment variable.
func (e *Command) AddEnvVar(envVar EnvVar) {
	e.Env = append(e.Env, envVar)
}

// AddEnvFrom adds environment variable sources.
func (e *Command) AddEnvFrom(envFrom []EnvFromSource) {
	e.EnvFrom = append(e.EnvFrom, envFrom...)
}

// GetEnv constructs environment variables for command execution.
func (e *Command) GetEnv() []string {
	// Start with a map to track environment variables.
	envMap := make(map[string]string)

	// Always start with essential environment variables for command execution.
	essentialVars := []string{"PATH", "HOME", "USER", "TERM", "COLORTERM"}
	for key, value := range e.baseEnv {
		// Keep essential environment variables.
		if slices.Contains(essentialVars, key) {
			envMap[key] = value
		}
	}

	// Apply envFrom config.
	e.applyEnvFrom(envMap)

	// Apply env config.
	e.applyEnv(envMap)

	// Convert map back to slice format.
	env := []string{}
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

func (e *Command) Exec(ctx context.Context, dir string) (*Result, error) {
	return e.ExecWithStdin(ctx, dir, nil)
}

func (e *Command) ExecWithStdin(ctx context.Context, dir string, stdin []byte) (*Result, error) {
	if e.Command == "" {
		return nil, ErrEmptyCommand
	}

	// Get environment variables for command execution.
	env := e.GetEnv()

	// Prepare the command to execute.
	//nolint:gosec // G204: Subprocess launched with a potential tainted input or cmd arguments.
	cmd := exec.CommandContext(ctx, e.Command, e.Args...)
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
		if stdout.Len() > 0 || stderr.Len() > 0 {
			return result, fmt.Errorf("%w: %w", ErrCommandExecution, err)
		}

		return nil, fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	slog.DebugContext(ctx, "command executed successfully")

	return result, nil
}

// CompilePatterns compiles all regex patterns.
func (e *Command) CompilePatterns() error {
	for i, envVar := range e.Env {
		if envVar.ValueFrom != nil && envVar.ValueFrom.CallerRef != nil {
			err := envVar.ValueFrom.CallerRef.Compile()
			if err != nil {
				return fmt.Errorf("env[%d]: %w", i, err)
			}
		}
	}

	for i, envFromSource := range e.EnvFrom {
		if envFromSource.CallerRef != nil {
			err := envFromSource.CallerRef.Compile()
			if err != nil {
				return fmt.Errorf("envFrom[%d]: %w", i, err)
			}
		}
	}

	return nil
}

func (e *Command) String() string {
	return fmt.Sprintf("%s %s", e.Command, strings.Join(e.Args, " "))
}

// applyEnvFrom applies all envFrom sources to the environment map.
func (e *Command) applyEnvFrom(envMap map[string]string) {
	for _, envFromSource := range e.EnvFrom {
		if envFromSource.CallerRef == nil {
			continue
		}

		// Handle pattern-based inheritance.
		pattern := envFromSource.CallerRef.compiledPattern
		if pattern != nil {
			for key, value := range e.baseEnv {
				if pattern.MatchString(key) {
					envMap[key] = value
				}
			}
		}

		// Handle name-based inheritance.
		nameRef := envFromSource.CallerRef.Name
		if nameRef != "" {
			if value, exists := e.baseEnv[nameRef]; exists {
				envMap[nameRef] = value
			}
		}
	}
}

// applyEnv applies environment variables from the env field.
func (e *Command) applyEnv(envMap map[string]string) {
	for _, envVar := range e.Env {
		if envVar.Name == "" {
			continue
		}

		if envVar.Value != "" {
			// Static value.
			envMap[envVar.Name] = envVar.Value

			continue
		}

		if envVar.ValueFrom != nil && envVar.ValueFrom.CallerRef != nil && envVar.ValueFrom.CallerRef.Name != "" {
			// Value from caller reference.
			if value, exists := envMap[envVar.ValueFrom.CallerRef.Name]; exists {
				envMap[envVar.Name] = value
			}
		}
	}
}
