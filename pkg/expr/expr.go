package expr

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
)

// Protect CEL environment creation and compilation from concurrent access.
var celMutex sync.Mutex

// Environment provides a thread-safe wrapper around a [*cel.Env].
type Environment struct {
	env *cel.Env
}

// NewEnvironment creates a new [Environment].
func NewEnvironment(opts ...cel.EnvOption) (*Environment, error) {
	env, err := createEnvironment(opts...)
	if err != nil {
		return nil, err
	}

	return &Environment{env: env}, nil
}

// MustNewEnvironment creates a new [Environment] and panics on error.
func MustNewEnvironment(opts ...cel.EnvOption) *Environment {
	env, err := NewEnvironment(opts...)
	if err != nil {
		panic(err)
	}

	return env
}

// ensureEnvironment creates the [*cel.Env] using the global mutex.
func createEnvironment(opts ...cel.EnvOption) (*cel.Env, error) {
	celMutex.Lock()
	defer celMutex.Unlock()

	opts = append(opts, cel.Lib(&lib{}))

	celEnv, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}

	return celEnv, nil
}

// Compile compiles a CEL expression and returns a program.
//
//nolint:ireturn // Following CEL's function signature.
func (e *Environment) Compile(expression string) (cel.Program, error) {
	celMutex.Lock()
	defer celMutex.Unlock()

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile expression: %w", issues.Err())
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("create program: %w", err)
	}

	return program, nil
}
