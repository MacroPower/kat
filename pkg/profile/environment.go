package profile

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// EnvFromSource represents a source for inheriting environment variables.
type EnvFromSource struct {
	CallerRef *CallerRef `yaml:"callerRef,omitempty"`
}

// CallerRef represents a reference to environment variables from the caller process.
type CallerRef struct {
	compiledPattern *regexp.Regexp // Compiled regex pattern for matching environment variables.

	Pattern string `yaml:"pattern,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

// EnvVar represents an environment variable definition.
type EnvVar struct {
	ValueFrom *EnvVarSource `yaml:"valueFrom,omitempty"`
	Name      string        `validate:"required"        yaml:"name"`
	Value     string        `yaml:"value,omitempty"`
}

// EnvVarSource represents a source for an environment variable value.
type EnvVarSource struct {
	CallerRef *CallerRef `yaml:"callerRef,omitempty"`
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

// Environment manages environment variables for command execution.
type Environment struct {
	baseEnv map[string]string
	Env     []EnvVar        `yaml:"env,omitempty"`
	EnvFrom []EnvFromSource `validate:"dive"      yaml:"envFrom,omitempty"`
}

// NewEnvironment creates a new [Environment].
// It accepts a base environment, which usually will be from [os.Environ].
func NewEnvironment(baseEnv []string) Environment {
	e := Environment{
		Env:     []EnvVar{},
		EnvFrom: []EnvFromSource{},
	}
	e.SetBaseEnv(baseEnv)

	return e
}

func (e *Environment) SetBaseEnv(baseEnv []string) {
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
func (e *Environment) AddEnvVar(envVar EnvVar) {
	e.Env = append(e.Env, envVar)
}

// AddEnvFrom adds environment variable sources.
func (e *Environment) AddEnvFrom(envFrom []EnvFromSource) {
	e.EnvFrom = append(e.EnvFrom, envFrom...)
}

// Build constructs the environment variables for command execution.
func (e *Environment) Build() []string {
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

// CompilePatterns compiles all regex patterns.
func (e *Environment) CompilePatterns() error {
	for i, envVar := range e.Env {
		if envVar.ValueFrom != nil && envVar.ValueFrom.CallerRef != nil {
			if err := envVar.ValueFrom.CallerRef.Compile(); err != nil {
				return fmt.Errorf("env[%d]: %w", i, err)
			}
		}
	}
	for i, envFromSource := range e.EnvFrom {
		if envFromSource.CallerRef != nil {
			if err := envFromSource.CallerRef.Compile(); err != nil {
				return fmt.Errorf("envFrom[%d]: %w", i, err)
			}
		}
	}

	return nil
}

// applyEnvFrom applies all envFrom sources to the environment map.
func (e *Environment) applyEnvFrom(envMap map[string]string) {
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
func (e *Environment) applyEnv(envMap map[string]string) {
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
