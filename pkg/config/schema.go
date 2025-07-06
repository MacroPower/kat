package config

import (
	"fmt"

	_ "embed"

	"github.com/macropower/kat/pkg/schema"
)

//go:generate go run ../../internal/schema_gen/main.go

//go:embed schema.json
var schemaJSON []byte

// ValidateWithSchema validates the given YAML data against the embedded JSON schema.
func ValidateWithSchema(data any) error {
	validator, err := schema.NewValidator(schemaJSON)
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	err = validator.Validate(data)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	return nil
}
