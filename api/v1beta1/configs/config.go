// Package configs provides the global Config configuration type for kat.
package configs

import (
	"fmt"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/api"
	"github.com/macropower/kat/api/v1beta1"
	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../../internal/schemagen/main.go -o configs.v1beta1.json

var (
	//go:embed config.yaml
	defaultConfigYAML []byte

	//go:embed configs.v1beta1.json
	schemaJSON []byte

	// ValidKinds contains the valid kind values for global configurations.
	ValidKinds = []string{"Configuration"}

	// DefaultValidator validates global configuration against the JSON schema.
	DefaultValidator = yaml.MustNewValidator("/configs.v1beta1.json", schemaJSON)

	// Compile-time interface checks.
	_ v1beta1.Object = (*Config)(nil)
)

// Config represents the global kat configuration.
//
//nolint:recvcheck // Must satisfy the jsonschema interface.
type Config struct {
	Command          *command.Config `json:",inline"`
	UI               *ui.Config      `json:",inline"`
	v1beta1.TypeMeta `json:",inline"`
}

// New creates a new global [Config] with default values.
func New() *Config {
	c := &Config{
		TypeMeta: v1beta1.TypeMeta{
			APIVersion: v1beta1.APIVersion,
			Kind:       "Configuration",
		},
	}
	c.EnsureDefaults()

	return c
}

// EnsureDefaults initializes nil fields to their default values.
func (c *Config) EnsureDefaults() {
	if c.UI == nil {
		c.UI = ui.NewConfig()
	} else {
		c.UI.EnsureDefaults()
	}

	if c.Command == nil {
		c.Command = command.NewConfig()
	} else {
		c.Command.EnsureDefaults()
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Command != nil {
		err := c.Command.Validate()
		if err != nil {
			return fmt.Errorf("validate command config: %w", err)
		}
	}

	return nil
}

func (c Config) JSONSchemaExtend(jss *jsonschema.Schema) {
	v1beta1.ExtendSchemaWithEnums(jss, v1beta1.ValidAPIVersions, ValidKinds)
}

// MarshalYAML serializes the config to YAML.
func (c Config) MarshalYAML() ([]byte, error) {
	type alias Config

	b, err := api.MarshalYAML(alias(c))
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	return b, nil
}

// Write writes the config to the specified path if it doesn't already exist.
func (c Config) Write(path string) error {
	b, err := c.MarshalYAML()
	if err != nil {
		return err
	}

	err = api.WriteIfNotExists(path, b)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// WriteDefault writes the embedded default config.yaml to the specified path.
func WriteDefault(path string, force bool) error {
	err := api.WriteDefaultFile(path, defaultConfigYAML, force, "configuration")
	if err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	return nil
}

// GetPath returns the path to the global configuration file.
func GetPath() string {
	return api.GetConfigPath("config.yaml")
}
