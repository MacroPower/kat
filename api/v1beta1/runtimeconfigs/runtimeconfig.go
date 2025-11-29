// Package runtimeconfigs provides the RuntimeConfig configuration type for kat.
package runtimeconfigs

import (
	"fmt"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/api"
	"github.com/macropower/kat/api/v1beta1"
	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../../internal/schemagen/project/main.go -o runtimeconfigs.v1beta1.json

var (
	// FileNames contains the valid names for runtime configuration files.
	FileNames = []string{
		".katrc.yaml",
		"katrc.yaml",
	}

	//go:embed runtimeconfigs.v1beta1.json
	runtimeSchemaJSON []byte

	// DefaultValidator validates runtime configuration against the JSON schema.
	DefaultValidator = yaml.MustNewValidator("/runtimeconfigs.v1beta1.json", runtimeSchemaJSON)

	// ValidKinds contains the valid kind values for runtime configurations.
	ValidKinds = []string{"RuntimeConfig"}

	// Compile-time interface checks.
	_ v1beta1.Object = (*RuntimeConfig)(nil)
)

// RuntimeConfig represents runtime-level configuration.
//
//nolint:recvcheck // Must satisfy the jsonschema interface.
type RuntimeConfig struct {
	Command          *command.Config `json:",inline"`
	v1beta1.TypeMeta `json:",inline"`
}

// New creates a new [RuntimeConfig].
func New() *RuntimeConfig {
	return &RuntimeConfig{
		TypeMeta: v1beta1.TypeMeta{
			APIVersion: v1beta1.APIVersion,
			Kind:       "RuntimeConfig",
		},
		Command: &command.Config{},
	}
}

// EnsureDefaults initializes nil fields to their default values.
func (c *RuntimeConfig) EnsureDefaults() {
	if c.Command == nil {
		c.Command = &command.Config{}
	}
}

// Validate validates the runtime configuration.
func (c *RuntimeConfig) Validate() error {
	if c.Command != nil {
		err := c.Command.Validate()
		if err != nil {
			return fmt.Errorf("validate command config: %w", err)
		}
	}

	return nil
}

func (c RuntimeConfig) JSONSchemaExtend(jss *jsonschema.Schema) {
	v1beta1.ExtendSchemaWithEnums(jss, v1beta1.ValidAPIVersions, ValidKinds)
}

// Find searches for a runtime config file starting from targetPath
// and walking up the directory tree until the filesystem root.
// It checks for all [FileNames] in each directory.
// Returns the path to the config file if found, or empty string if not found.
func Find(targetPath string) (string, error) {
	path, err := api.FindConfigFile(targetPath, FileNames)
	if err != nil {
		return "", fmt.Errorf("find config file: %w", err)
	}

	return path, nil
}
