package yaml

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
	"golang.org/x/tools/go/packages"
)

type LookupCommentFunc func(commentMap map[string]string) func(t reflect.Type, f string) string

// SchemaGenerator generates a JSON schema from a Go type using reflection.
// It looks up comments from the source code to provide documentation, one or
// more package paths are provided. Uses [github.com/invopop/jsonschema].
type SchemaGenerator struct {
	Reflector         *jsonschema.Reflector
	LookupCommentFunc LookupCommentFunc
	reflectTarget     any
	packagePaths      []string
	Tests             bool // Include test files.
}

// NewSchemaGenerator creates a new [SchemaGenerator].
// The reflectTarget is the Go type to generate the schema for, and packagePaths
// are the fully qualified import paths of the packages to lookup comments from.
func NewSchemaGenerator(reflectTarget any, packagePaths ...string) *SchemaGenerator {
	return &SchemaGenerator{
		Reflector:         new(jsonschema.Reflector),
		LookupCommentFunc: DefaultLookupCommentFunc,
		packagePaths:      packagePaths,
		reflectTarget:     reflectTarget,
	}
}

func (g *SchemaGenerator) Generate() ([]byte, error) {
	if len(g.packagePaths) > 0 {
		err := g.addLookupComment()
		if err != nil {
			return nil, fmt.Errorf("lookup comments: %w", err)
		}
	}

	js := g.Reflector.Reflect(g.reflectTarget)
	jsData, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return jsData, nil
}

func (g *SchemaGenerator) addLookupComment() error {
	// Find the module root directory.
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return fmt.Errorf("find module root: %w", err)
	}

	commentMap := make(map[string]string)

	// Load packages using the modern packages API.
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
		Dir:   moduleRoot,
		Tests: g.Tests,
	}

	pkgs, err := packages.Load(cfg, g.packagePaths...)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
	}

	// Check for package loading errors.
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Skip packages with errors.
			continue
		}
		// Build comment map for this package.
		buildCommentMapForPackage(pkg, commentMap)
	}

	// AdditionalProperties are not set correctly when references are used.
	// So, we hardcode this value to false for now.
	g.Reflector.DoNotReference = true

	// Create and set a lookup function that uses the comment map.
	g.Reflector.LookupComment = g.LookupCommentFunc(commentMap)

	return nil
}

func DefaultLookupCommentFunc(commentMap map[string]string) func(t reflect.Type, f string) string {
	return func(t reflect.Type, f string) string {
		typeName := t.Name()
		pkgPath := t.PkgPath()

		// Generate the documentation URL.
		var docURL string
		if f == "" {
			docURL = fmt.Sprintf("%s: https://pkg.go.dev/%s#%s", typeName, pkgPath, typeName)
		} else {
			docURL = fmt.Sprintf("%s.%s: https://pkg.go.dev/%s#%s", typeName, f, pkgPath, typeName)
		}

		// Look up the comment from the parsed source code.
		var comment string
		if f == "" {
			// Type comment - use the package name from the path.
			pkgName := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
			if typeComment, ok := commentMap[pkgName+"."+typeName]; ok {
				comment = typeComment
			}
		} else {
			// Field comment - use the package name from the path.
			pkgName := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
			if fieldComment, ok := commentMap[pkgName+"."+typeName+"."+f]; ok {
				comment = fieldComment
			}
		}

		// Combine the documentation URL with the comment if available.
		if comment != "" {
			return fmt.Sprintf("%s\n\n%s", comment, docURL)
		}

		return docURL
	}
}

// findModuleRoot searches for the go.mod file starting from the current directory and going up.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory.
			break
		}

		dir = parent
	}

	return "", errors.New("go.mod not found")
}

// buildCommentMapForPackage parses Go packages and builds a map of comments for types and fields.
func buildCommentMapForPackage(pkg *packages.Package, commentMap map[string]string) {
	// Walk through all syntax files to extract type and field comments directly.
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				// Extract type comments from general declarations.
				if node.Doc != nil {
					for _, spec := range node.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							key := pkg.Name + "." + typeSpec.Name.Name
							commentMap[key] = cleanComment(node.Doc.Text())
						}
					}
				}

			case *ast.TypeSpec:
				// Extract type comments from individual type specs.
				if node.Doc != nil {
					key := pkg.Name + "." + node.Name.Name
					commentMap[key] = cleanComment(node.Doc.Text())
				}

				// Extract field comments from struct types.
				if structType, ok := node.Type.(*ast.StructType); ok {
					typeName := node.Name.Name
					for _, field := range structType.Fields.List {
						// Check both Doc (leading comments) and Comment (trailing comments).
						var comment string
						if field.Doc != nil {
							comment = field.Doc.Text()
						} else if field.Comment != nil {
							comment = field.Comment.Text()
						}

						if comment != "" && len(field.Names) > 0 {
							fieldName := field.Names[0].Name
							key := pkg.Name + "." + typeName + "." + fieldName
							commentMap[key] = cleanComment(comment)
						}
					}
				}
			}

			return true
		})
	}
}

// cleanComment removes leading comment markers and extra whitespace.
func cleanComment(comment string) string {
	lines := strings.Split(comment, "\n")
	cleanLines := []string{}

	for _, line := range lines {
		line = strings.TrimPrefix(line, "//")
		cleanLines = append(cleanLines, line)
	}

	return strings.TrimSpace(strings.Join(cleanLines, "\n"))
}
