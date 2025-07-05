package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/macropower/kat/pkg/config"
)

func main() {
	// Find the module root directory.
	moduleRoot, err := findModuleRoot()
	if err != nil {
		panic(fmt.Errorf("find module root: %w", err))
	}

	// Parse all relevant packages to extract struct field comments.
	fset := token.NewFileSet()
	commentMap := make(map[string]string)

	// List of packages to parse for comments (relative to module root).
	packagePaths := []string{
		"pkg/config",
		"pkg/command",
		"pkg/ui",
		"pkg/profile",
		"pkg/rule",
		"pkg/execs",
		"pkg/keys",
	}

	for _, pkgPath := range packagePaths {
		fullPath := filepath.Join(moduleRoot, pkgPath)
		pkgs, err := parser.ParseDir(fset, fullPath, nil, parser.ParseComments)
		if err != nil {
			// Skip packages that don't exist or can't be parsed.
			continue
		}

		// Build comment map for this package.
		buildCommentMapForPackage(fset, pkgs, commentMap)
	}

	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	r.ExpandedStruct = true
	r.LookupComment = func(t reflect.Type, f string) string {
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

	js := r.Reflect(config.NewConfig())
	jsData, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		panic(fmt.Errorf("marshal JSON schema: %w", err))
	}

	// Write schema.json file.
	if err := os.WriteFile("schema.json", jsData, 0o600); err != nil {
		panic(fmt.Errorf("write schema file: %w", err))
	}
}

// findModuleRoot searches for the go.mod file starting from the current directory and going up.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
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
func buildCommentMapForPackage(fset *token.FileSet, pkgs map[string]*ast.Package, commentMap map[string]string) {
	for _, pkg := range pkgs {
		// Create documentation from the package.
		docPkg := doc.New(pkg, "./", 0)

		// Extract type comments.
		for _, typ := range docPkg.Types {
			if typ.Doc != "" {
				key := pkg.Name + "." + typ.Name
				commentMap[key] = cleanComment(typ.Doc)
			}
		}

		// Walk through all files to extract field comments.
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.TypeSpec:
					if structType, ok := node.Type.(*ast.StructType); ok {
						typeName := node.Name.Name

						// Extract field comments.
						for _, field := range structType.Fields.List {
							if field.Comment != nil && len(field.Names) > 0 {
								fieldName := field.Names[0].Name
								comment := field.Comment.Text()
								if comment != "" {
									key := pkg.Name + "." + typeName + "." + fieldName
									commentMap[key] = cleanComment(comment)
								}
							}
						}
					}
				}

				return true
			})
		}
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
