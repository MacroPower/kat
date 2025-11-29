// Package projectconfigs provides the ProjectConfig configuration type for kat.
package projectconfigs

import (
	"fmt"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/api"
	"github.com/macropower/kat/api/v1beta1"
	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../../internal/schemagen/project/main.go -o projectconfigs.v1beta1.json

var (
	// FileNames contains the valid names for project configuration files.
	FileNames = []string{
		".kat.yaml",
		"kat.yaml",
	}

	//go:embed projectconfigs.v1beta1.json
	projectSchemaJSON []byte

	// DefaultValidator validates project configuration against the JSON schema.
	DefaultValidator = yaml.MustNewValidator("/projectconfigs.v1beta1.json", projectSchemaJSON)

	// ValidKinds contains the valid kind values for project configurations.
	ValidKinds = []string{"ProjectConfig"}

	// Compile-time interface checks.
	_ v1beta1.Object = (*ProjectConfig)(nil)
)

// ProjectConfig represents project-level configuration.
//
//nolint:recvcheck // Must satisfy the jsonschema interface.
type ProjectConfig struct {
	Command          *command.Config `json:",inline"`
	v1beta1.TypeMeta `json:",inline"`
}

// New creates a new [ProjectConfig].
func New() *ProjectConfig {
	return &ProjectConfig{
		TypeMeta: v1beta1.TypeMeta{
			APIVersion: v1beta1.APIVersion,
			Kind:       "ProjectConfig",
		},
		Command: &command.Config{},
	}
}

// EnsureDefaults initializes nil fields to their default values.
func (c *ProjectConfig) EnsureDefaults() {
	if c.Command == nil {
		c.Command = &command.Config{}
	}
}

// Validate validates the project configuration.
func (c *ProjectConfig) Validate() error {
	if c.Command != nil {
		err := c.Command.Validate()
		if err != nil {
			return fmt.Errorf("validate command config: %w", err)
		}
	}

	return nil
}

func (c ProjectConfig) JSONSchemaExtend(jss *jsonschema.Schema) {
	v1beta1.ExtendSchemaWithEnums(jss, v1beta1.ValidAPIVersions, ValidKinds)
}

// Find searches for a project config file starting from targetPath
// and walking up the directory tree until the filesystem root.
// It checks for all [FileNames] in each directory.
// Returns the path to the config file if found, or empty string if not found.
func Find(targetPath string) (string, error) {
	path, err := api.FindConfigFile(targetPath, FileNames)
	if err != nil {
		return "", fmt.Errorf("find project config: %w", err)
	}

	return path, nil
}
