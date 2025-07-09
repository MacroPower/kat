package expr

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

var (
	// Protect CEL environment creation from concurrent access.
	celMutex sync.Mutex

	// DefaultEnvironment is a shared environment instance for simple use cases.
	// This allows multiple compilations to reuse the same environment efficiently.
	DefaultEnvironment = MustNewEnvironment()
)

// Environment provides a thread-safe wrapper around a [*cel.Env].
type Environment struct {
	env *cel.Env
	mu  sync.Mutex
}

// NewEnvironment creates a new [Environment].
func NewEnvironment() (*Environment, error) {
	env, err := createEnvironment()
	if err != nil {
		return nil, err
	}

	return &Environment{env: env}, nil
}

// MustNewEnvironment creates a new [Environment] and panics on error.
func MustNewEnvironment() *Environment {
	env, err := NewEnvironment()
	if err != nil {
		panic(err)
	}

	return env
}

// ensureEnvironment creates the [*cel.Env] using the global mutex.
func createEnvironment() (*cel.Env, error) {
	celMutex.Lock()
	defer celMutex.Unlock()

	celEnv, err := cel.NewEnv(
		cel.Variable("files", cel.ListType(cel.StringType)),
		cel.Variable("dir", cel.StringType),

		// `pathBase` returns the last element of the path.
		// Example: files.filter(f, pathBase(f) in ["Chart.yaml", "values.yaml"]).
		cel.Function("pathBase",
			cel.Overload("path_base", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(path ref.Val) ref.Val {
					pathValue, ok := path.(types.String).Value().(string)
					if !ok {
						return types.NewErr("pathBase: invalid string value")
					}

					return types.String(filepath.Base(pathValue))
				}),
			),
		),

		// `pathDir` returns all but the last element of the path.
		// Example: files.filter(f, pathDir(f).contains("/templates/")).
		cel.Function("pathDir",
			cel.Overload("path_dir", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(path ref.Val) ref.Val {
					pathValue, ok := path.(types.String).Value().(string)
					if !ok {
						return types.NewErr("pathDir: invalid string value")
					}

					return types.String(filepath.Dir(pathValue))
				}),
			),
		),

		// `pathExt` returns the file extension of the path.
		// Example: files.filter(f, pathExt(f) in [".yaml", ".yml"]).
		cel.Function("pathExt",
			cel.Overload("path_ext", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(path ref.Val) ref.Val {
					pathValue, ok := path.(types.String).Value().(string)
					if !ok {
						return types.NewErr("pathExt: invalid string value")
					}

					return types.String(filepath.Ext(pathValue))
				}),
			),
		),

		// `yamlPath` reads a YAML file and extracts a value using a YAML path.
		// Returns the value at the specified path, or null if the path doesn't exist or file can't be read.
		// Example: files.filter(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2").
		cel.Function("yamlPath",
			cel.Overload("yaml_path", []*cel.Type{cel.StringType, cel.StringType}, cel.DynType,
				cel.BinaryBinding(func(filePath, yamlPathExpr ref.Val) ref.Val {
					filePathStr, ok := filePath.(types.String).Value().(string)
					if !ok {
						return types.NewErr("yamlPath: invalid file path")
					}

					yamlPathStr, ok := yamlPathExpr.(types.String).Value().(string)
					if !ok {
						return types.NewErr("yamlPath: invalid yaml path")
					}

					logger := slog.With(
						slog.String("file", filePathStr),
						slog.String("yamlPath", yamlPathStr),
					)

					// Read file content.
					//nolint:gosec // G304: Potential file inclusion via variable.
					content, err := os.ReadFile(filePathStr)
					if err != nil {
						// Return null if file can't be read, don't error.
						logger.Debug("failed to read YAML file, returning null",
							slog.Any("error", err),
						)

						return types.NullValue
					}

					// Parse YAML path.
					path, err := yaml.PathString(yamlPathStr)
					if err != nil {
						// Return null if path is invalid.
						logger.Debug("invalid YAML path, returning null",
							slog.Any("error", err),
						)

						return types.NullValue
					}

					// Extract value using YAML path.
					var value any

					err = path.Read(strings.NewReader(string(content)), &value)
					if err != nil {
						// Return null if path doesn't exist or extraction fails.
						logger.Debug("failed to extract value from YAML, returning null",
							slog.Any("error", err),
						)

						return types.NullValue
					}

					// Convert the extracted value to a CEL value.
					return ConvertToCELValue(value)
				}),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}

	return celEnv, nil
}

// Compile compiles a CEL expression and returns a program.
//
//nolint:ireturn // Following CEL's function signature.
func (e *Environment) Compile(expression string) (cel.Program, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

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

// ConvertToCELValue converts a Go value to a CEL value.
// Handles common YAML types and returns null for unsupported types.
//
//nolint:ireturn // Following CEL's function signature.
func ConvertToCELValue(value any) ref.Val {
	switch v := value.(type) {
	case nil:
		return types.NullValue

	case bool:
		return types.Bool(v)

	case int:
		return types.Int(v)

	case int8:
		return types.Int(int64(v))

	case int16:
		return types.Int(int64(v))

	case int32:
		return types.Int(int64(v))

	case int64:
		return types.Int(v)

	case uint:
		// Check for overflow when converting to int64.
		if v > math.MaxInt64 {
			return types.Double(float64(v))
		}

		return types.Int(int64(v))

	case uint8:
		return types.Int(int64(v))

	case uint16:
		return types.Int(int64(v))

	case uint32:
		return types.Int(int64(v))

	case uint64:
		// Check for overflow when converting to int64.
		if v > math.MaxInt64 {
			return types.Double(float64(v))
		}

		return types.Int(int64(v))

	case float32:
		return types.Double(float64(v))

	case float64:
		return types.Double(v)

	case string:
		return types.String(v)

	case []any:
		// Convert slice to CEL list.
		celValues := make([]ref.Val, len(v))
		for i, item := range v {
			celValues[i] = ConvertToCELValue(item)
		}

		return types.NewDynamicList(types.DefaultTypeAdapter, celValues)

	case map[any]any:
		// Convert map to CEL map.
		celMap := make(map[ref.Val]ref.Val)
		for key, val := range v {
			celKey := ConvertToCELValue(key)
			celVal := ConvertToCELValue(val)
			celMap[celKey] = celVal
		}

		return types.NewDynamicMap(types.DefaultTypeAdapter, celMap)

	case map[string]any:
		// Convert string map to CEL map.
		celMap := make(map[ref.Val]ref.Val)
		for key, val := range v {
			celKey := types.String(key)
			celVal := ConvertToCELValue(val)
			celMap[celKey] = celVal
		}

		return types.NewDynamicMap(types.DefaultTypeAdapter, celMap)

	default:
		// For unsupported types, return null instead of erroring.
		return types.NullValue
	}
}
