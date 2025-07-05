package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"

	_ "embed"
)

//go:generate go run ../../internal/schema_gen/main.go

//go:embed schema.json
var schemaJSON []byte

// SchemaValidationError represents a validation error from JSON schema validation.
// It wraps the original validation result and provides path information for [yaml.Path.AnnotateSource].
type SchemaValidationError struct {
	Path   *yaml.Path // YAML path to the validation error.
	Err    error      // Underlying error.
	Field  string     // Field name that failed validation.
	Detail string     // Detailed error message.
}

func (e SchemaValidationError) Error() string {
	if e.Path != nil {
		return fmt.Sprintf("validation error at %s: %s", e.Path.String(), e.Detail)
	}

	return "validation error: " + e.Detail
}

// ValidateWithSchema validates the given YAML data against the embedded JSON schema.
// It returns a [SchemaValidationError] that can be used with [yaml.Path.AnnotateSource] for precise error reporting.
func ValidateWithSchema(data []byte) error {
	// Parse the schema.
	var schemaData any
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	// Create a compiler and add the schema.
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaData); err != nil {
		return fmt.Errorf("add schema resource: %w", err)
	}

	// Compile the schema.
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	// Convert YAML to JSON for validation.
	var yamlData any
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	// Validate against schema.
	err = schema.Validate(yamlData)
	if err == nil {
		return nil
	}

	// Convert validation error to our custom error type with path information.
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return fmt.Errorf("schema validation: %w", err)
	}

	// Build the path from the validation error.
	path, pathErr := buildYAMLPathFromError(validationErr)
	if pathErr != nil {
		// If we can't build the path, still return a useful error.
		return &SchemaValidationError{
			Err:    errors.New("schema validation"),
			Detail: validationErr.Error(),
		}
	}

	return &SchemaValidationError{
		Path:   path,
		Err:    errors.New("schema validation"),
		Detail: validationErr.Error(),
	}
}

// buildYAMLPathFromError converts a JSON schema validation error to a [yaml.Path].
// The jsonschema/v6 library provides path information in the InstanceLocation field of nested causes.
func buildYAMLPathFromError(validationErr *jsonschema.ValidationError) (*yaml.Path, error) {
	// Find the cause with the most specific (longest) InstanceLocation.
	mostSpecificLocation := findMostSpecificLocation(validationErr)

	return buildPathFromLocation(mostSpecificLocation)
}

// findMostSpecificLocation recursively searches through all causes to find the one with the longest InstanceLocation.
func findMostSpecificLocation(err *jsonschema.ValidationError) []string {
	longest := err.InstanceLocation

	// Recursively check all causes.
	for _, cause := range err.Causes {
		candidateLocation := findMostSpecificLocation(cause)
		if len(candidateLocation) > len(longest) {
			longest = candidateLocation
		}
	}

	return longest
}

// buildPathFromLocation converts an InstanceLocation slice to a [yaml.Path].
func buildPathFromLocation(location []string) (*yaml.Path, error) {
	if len(location) == 0 {
		// Root level error.
		pb := yaml.PathBuilder{}

		return pb.Root().Build(), nil
	}

	pb := yaml.PathBuilder{}
	current := pb.Root()

	for _, part := range location {
		// Check if this part is a numeric index.
		var index uint
		if _, err := fmt.Sscanf(part, "%d", &index); err == nil {
			// This is an array index.
			current = current.Index(index)
		} else {
			// Regular property name.
			current = current.Child(part)
		}
	}

	return current.Build(), nil
}
