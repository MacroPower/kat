// Package expr provides CEL (Common Expression Language) functionality
// for evaluating expressions against file paths and YAML content.
//
// It creates CEL environments with custom functions for:
//   - File path operations (pathBase, pathDir, pathExt)
//   - YAML content extraction (yamlPath)
//
// CEL expressions have access to variables:
//   - `files` (list<string>): All file paths in directory
//   - `dir` (string): The directory path being processed
package expr
