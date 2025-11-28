package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../internal/schemagen/project/main.go -o projectconfigurations.v1beta1.json

var (
	// ProjectConfigFileNames contains the valid names for project configuration files.
	ProjectConfigFileNames = []string{
		".katrc.yaml",
		"katrc.yaml",
	}

	//go:embed projectconfigurations.v1beta1.json
	projectSchemaJSON []byte

	// ProjectValidator validates project configuration against the JSON schema.
	ProjectValidator = yaml.MustNewValidator("/projectconfigurations.v1beta1.json", projectSchemaJSON)

	// ValidProjectKinds contains the valid kind values for project configurations.
	ValidProjectKinds = []string{
		"ProjectConfiguration",
	}
)

// ProjectConfig represents project-level configuration.
type ProjectConfig struct {
	Command *command.Config `json:",inline"`
	// APIVersion specifies the API version for this configuration.
	APIVersion string `json:"apiVersion" jsonschema:"title=API Version"`
	// Kind defines the type of configuration.
	Kind string `json:"kind" jsonschema:"title=Kind"`
}

// NewProjectConfig creates a new [ProjectConfig].
func NewProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "ProjectConfiguration",
		Command:    &command.Config{},
	}
}

func (c ProjectConfig) JSONSchemaExtend(jss *jsonschema.Schema) {
	extendSchemaWithEnums(jss, ValidAPIVersions, ValidProjectKinds)
}

// ProjectConfigLoader loads and validates project configuration files.
type ProjectConfigLoader struct {
	*baseLoader
}

// NewProjectConfigLoaderFromBytes creates a ProjectConfigLoader from byte data.
func NewProjectConfigLoaderFromBytes(data []byte, opts ...ConfigLoaderOpt) *ProjectConfigLoader {
	return &ProjectConfigLoader{baseLoader: newBaseLoader(data, ProjectValidator, opts...)}
}

// NewProjectConfigLoaderFromFile creates a ProjectConfigLoader from a file path.
func NewProjectConfigLoaderFromFile(path string, opts ...ConfigLoaderOpt) (*ProjectConfigLoader, error) {
	data, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("read project config file: %w", err)
	}

	return NewProjectConfigLoaderFromBytes(data, opts...), nil
}

// Load parses and returns the ProjectConfig.
func (pcl *ProjectConfigLoader) Load() (*ProjectConfig, error) {
	c := &ProjectConfig{}
	dec := yaml.NewDecoder(bytes.NewReader(pcl.data))

	err := dec.Decode(c)
	if err != nil {
		return nil, pcl.yamlError.Wrap(err)
	}

	if c.Command == nil {
		c.Command = &command.Config{}
	}

	err = c.Command.Validate()
	if err != nil {
		return nil, pcl.yamlError.Wrap(err)
	}

	return c, nil
}

// FindProjectConfig searches for a project config file starting from targetPath
// and walking up the directory tree until the filesystem root.
// It checks for all [ProjectConfigFileNames] in each directory.
// Returns the path to the config file if found, or empty string if not found.
func FindProjectConfig(targetPath string) (string, error) {
	// Get absolute path.
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}

	// If targetPath is a file, start from its directory.
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}

	var searchDir string
	if info.IsDir() {
		searchDir = absPath
	} else {
		searchDir = filepath.Dir(absPath)
	}

	// Walk up the directory tree looking for project config files.
	for {
		for _, fileName := range ProjectConfigFileNames {
			configPath := filepath.Join(searchDir, fileName)

			_, statErr := os.Stat(configPath)
			if statErr == nil {
				return configPath, nil
			}
		}

		// Move to parent directory.
		parent := filepath.Dir(searchDir)
		if parent == searchDir {
			// Reached the root, no config found.
			break
		}

		searchDir = parent
	}

	return "", nil
}
