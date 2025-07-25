package yaml

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates data against a JSON schema.
// Uses [github.com/santhosh-tekuri/jsonschema/v6].
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator creates a new [Validator] with the provided JSON schema data.
func NewValidator(url string, schemaData []byte) (*Validator, error) {
	var schema any

	err := json.Unmarshal(schemaData, &schema)
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	err = compiler.AddResource(url, schema)
	if err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	jss, err := compiler.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	return &Validator{schema: jss}, nil
}

func MustNewValidator(url string, schemaData []byte) *Validator {
	v, err := NewValidator(url, schemaData)
	if err != nil {
		panic(err)
	}

	return v
}

// ValidateWithSchema validates the given data against the schema.
// It returns a [ValidationError] that can be used with [yaml.Path.AnnotateSource]
// for precise error reporting.
func (s *Validator) Validate(data any) error {
	// Validate against schema.
	err := s.schema.Validate(data)
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
		return &Error{
			Err: fmt.Errorf("schema validation: %w", validationErr),
		}
	}

	return &Error{
		Err:  validationErr,
		Path: path,
	}
}

// buildYAMLPathFromError extracts creates a [yaml.Path] from the provided
// [jsonschema.ValidationError].
func buildYAMLPathFromError(validationErr *jsonschema.ValidationError) (*yaml.Path, error) {
	// Find the cause with the most specific (longest) InstanceLocation.
	mostSpecificLocation := findMostSpecificLocation(validationErr)

	return buildPathFromLocation(mostSpecificLocation)
}

// findMostSpecificLocation recursively searches through all causes to find the
// one with the longest InstanceLocation.
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
		return NewPathBuilder().Root().Build(), nil
	}

	pb := NewPathBuilder()
	current := pb.Root()

	for _, part := range location {
		// Check if this part is a numeric index.
		var index uint

		_, err := fmt.Sscanf(part, "%d", &index)
		if err == nil {
			// This is an array index.
			current = current.Index(index)
		} else {
			// Regular property name.
			current = current.Child(part)
		}
	}

	return current.Build(), nil
}
