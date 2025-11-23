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

// LazyProgram provides thread-safe lazy compilation of a CEL expression.
// The expression is compiled at most once, even when accessed concurrently.
type LazyProgram struct {
	program    cel.Program
	err        error
	env        *Environment
	expression string
	once       sync.Once
}

// NewLazyProgram creates a new LazyProgram that will compile the given expression
// using the provided environment when Get() is first called.
func NewLazyProgram(expression string, env *Environment) *LazyProgram {
	return &LazyProgram{
		expression: expression,
		env:        env,
	}
}

// Get returns the compiled program, compiling it on the first call.
// Subsequent calls return the cached result.
//
//nolint:ireturn // Following CEL's function signature.
func (lp *LazyProgram) Get() (cel.Program, error) {
	lp.once.Do(func() {
		if lp.expression == "" {
			return
		}

		lp.program, lp.err = lp.env.Compile(lp.expression)
	})

	return lp.program, lp.err
}

// IsCompiled returns true if the program has been compiled.
func (lp *LazyProgram) IsCompiled() bool {
	return lp.program != nil || lp.err != nil
}
