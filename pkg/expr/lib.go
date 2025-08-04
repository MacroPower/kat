package expr

import (
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/goccy/go-yaml"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
)

type lib struct{}

func (lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		ext.Math(),
		ext.Strings(),
		ext.Lists(),

		cel.Constant("fs.CREATE", types.IntType, types.Int(fsnotify.Create)),
		cel.Constant("fs.REMOVE", types.IntType, types.Int(fsnotify.Remove)),
		cel.Constant("fs.WRITE", types.IntType, types.Int(fsnotify.Write)),
		cel.Constant("fs.RENAME", types.IntType, types.Int(fsnotify.Rename)),
		cel.Constant("fs.CHMOD", types.IntType, types.Int(fsnotify.Chmod)),

		// `has` macro and function for checking if an event has specific flags.
		// Example: op.has(fs.CREATE).
		// Example: op.has(fs.CREATE, fs.RENAME, fs.REMOVE).
		cel.Macros(
			cel.ReceiverVarArgMacro("has", hasVarArgMacro),
		),
		cel.Function("@has",
			cel.Overload("@has_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(event, flag ref.Val) ref.Val {
					eventInt, ok := event.(types.Int).Value().(int64)
					if !ok {
						return types.NewErr("has: invalid event value")
					}
					if eventInt > math.MaxUint32 {
						return types.NewErr("has: event value out of range")
					}

					eventValue := fsnotify.Op(eventInt) //nolint:gosec // G115: integer overflow conversion.

					flagInt, ok := flag.(types.Int).Value().(int64)
					if !ok {
						return types.NewErr("has: invalid flag value")
					}
					if flagInt > math.MaxUint32 {
						return types.NewErr("has: flag value out of range")
					}

					flagValue := fsnotify.Op(flagInt) //nolint:gosec // G115: integer overflow conversion.

					return types.Bool(eventValue.Has(flagValue))
				}),
			),
			cel.Overload("@has_int_list_int", []*cel.Type{cel.IntType, cel.ListType(cel.IntType)}, cel.BoolType,
				cel.BinaryBinding(func(event, flags ref.Val) ref.Val {
					eventInt, ok := event.(types.Int).Value().(int64)
					if !ok {
						return types.NewErr("has: invalid event value")
					}
					if eventInt > math.MaxUint32 {
						return types.NewErr("has: event value out of range")
					}

					eventValue := fsnotify.Op(eventInt) //nolint:gosec // G115: integer overflow conversion.

					flagsList, ok := flags.(traits.Lister)
					if !ok {
						return types.NewErr("has: invalid flags list")
					}

					flagSize, ok := flagsList.Size().(types.Int)
					if !ok {
						return types.NewErr("has: invalid flags list size")
					}

					// Check if the event has any of the specified flags.
					var mask int64
					for i := range flagSize {
						flagVal := flagsList.Get(i)
						flagInt, ok := flagVal.(types.Int).Value().(int64)
						if !ok {
							return types.NewErr("has: invalid flag value in list")
						}

						mask |= flagInt
					}
					if mask > math.MaxUint32 {
						return types.NewErr("has: flag value out of range")
					}

					//nolint:gosec // G115: integer overflow conversion.
					return types.Bool(eventValue.Has(fsnotify.Op(mask)))
				}),
			),
		),

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
	}
}

func (lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

//nolint:ireturn // Following CEL's function signature.
func hasVarArgMacro(meh cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
	switch len(args) {
	case 0:
		return nil, meh.NewError(target.ID(), "has() requires at least one argument")
	case 1:
		return meh.NewCall("@has", target, args[0]), nil
	default:
		return meh.NewCall("@has", target, meh.NewList(args...)), nil
	}
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
