package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/invopop/jsonschema"

	_ "embed"

	yaml "github.com/goccy/go-yaml"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/schema"
	"github.com/macropower/kat/pkg/ui"
)

var (
	//go:embed config.yaml
	defaultConfigYAML []byte

	ValidAPIVersions = []string{
		"kat.jacobcolvin.com/v1beta1",
	}
	ValidKinds = []string{
		"Configuration",
	}
)

//nolint:recvcheck // Must satisfy the jsonschema interface.
type Config struct {
	Command *command.Config `json:",inline"`
	UI      *ui.Config      `json:",inline"`
	// APIVersion specifies the API version for this configuration.
	APIVersion string `json:"apiVersion" jsonschema:"title=API Version"`
	// Kind defines the type of configuration.
	Kind string `json:"kind" jsonschema:"title=Kind"`
}

func NewConfig() *Config {
	c := &Config{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "Configuration",
	}
	c.EnsureDefaults()

	return c
}

func (c *Config) EnsureDefaults() {
	if c.UI == nil {
		c.UI = ui.DefaultConfig
	} else {
		c.UI.EnsureDefaults()
	}

	if c.Command == nil {
		c.Command = command.DefaultConfig
	} else {
		c.Command.EnsureDefaults()
	}
}

func (c Config) JSONSchemaExtend(jss *jsonschema.Schema) {
	apiVersion, ok := jss.Properties.Get("apiVersion")
	if !ok {
		panic("apiVersion property not found in schema")
	}

	for _, version := range ValidAPIVersions {
		apiVersion.OneOf = append(apiVersion.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: version,
			Title: "API Version",
		})
	}

	_, _ = jss.Properties.Set("apiVersion", apiVersion)

	kind, ok := jss.Properties.Get("kind")
	if !ok {
		panic("kind property not found in schema")
	}

	for _, kindValue := range ValidKinds {
		kind.OneOf = append(kind.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: kindValue,
			Title: "Kind",
		})
	}

	_, _ = jss.Properties.Set("kind", kind)
}

func ReadConfig(path string) ([]byte, error) {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.IsDir() {
			return nil, fmt.Errorf("%s: path is a directory", path)
		}
		if err == nil && !pathInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: unknown file state", path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: Potential file inclusion via variable.
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return data, nil
}

func LoadConfig(data []byte) (*Config, error) {
	reader := bytes.NewReader(data)

	// Decode into interface{} for schema validation.
	var anyConfig any

	dec := yaml.NewDecoder(reader, yaml.AllowDuplicateMapKey())
	err := dec.Decode(&anyConfig)
	if err != nil {
		return nil, fmt.Errorf("decode yaml config: %w", err)
	}

	// Validate against JSON schema.
	err = ValidateWithSchema(anyConfig)
	if err != nil {
		schemaErr := &schema.ValidationError{}
		if errors.As(err, &schemaErr) {
			source, srcErr := schemaErr.Path.AnnotateSource(data, true)
			if srcErr != nil {
				return nil, fmt.Errorf("path annotation failed: %w; caused by: %w", srcErr, err)
			}

			return nil, fmt.Errorf("%w\n%s", err, source)
		}

		return nil, fmt.Errorf("schema: %w", err)
	}

	// Validation passed; reset reader and decode into a Config struct.
	_, err = reader.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("prepare reader for decoding yaml config: %w", err)
	}

	c := &Config{}
	dec = yaml.NewDecoder(reader, yaml.AllowDuplicateMapKey())
	err = dec.Decode(c)
	if err != nil {
		return nil, fmt.Errorf("decode yaml config: %w", err)
	}

	c.EnsureDefaults()

	// Run Go validation on the config (for requirements that can't be represented in the schema).
	cfgErr := c.Command.Validate()
	if cfgErr != nil {
		source, srcErr := cfgErr.Path.AnnotateSource(data, true)
		if srcErr != nil {
			slog.Warn("failed to annotate config with error",
				slog.String("path", cfgErr.Path.String()),
				slog.Any("error", srcErr),
			)

			return nil, fmt.Errorf("validate config: %w", cfgErr)
		}

		return nil, fmt.Errorf("validate config: %w\n%s", cfgErr, source)
	}

	return c, nil
}

func (c *Config) Write(path string) error {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.Mode().IsRegular() {
			return nil // Config already exists.
		}
		if pathInfo.IsDir() {
			return fmt.Errorf("%s: path is a directory", path)
		}

		return fmt.Errorf("%s: unknown file state", path)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	b, err := c.MarshalYAML()
	if err != nil {
		return err
	}

	err = os.WriteFile(path, b, 0o600)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// WriteDefaultConfig writes the embedded default config.yaml and jsonschema to
// the specified path.
func WriteDefaultConfig(path string, force bool) error {
	configExists := false
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		switch {
		case err == nil && pathInfo.Mode().IsRegular():
			configExists = true
		case pathInfo.IsDir():
			return fmt.Errorf("%s: path is a directory", path)
		default:
			return fmt.Errorf("%s: unknown file state", path)
		}
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	if configExists && force {
		// Move the existing file to a backup.
		backupFile := fmt.Sprintf("%s.%d.old", filepath.Base(path), time.Now().UnixNano())
		backupPath := filepath.Join(filepath.Dir(path), backupFile)
		slog.Info("backing up existing config file",
			slog.String("path", backupPath),
		)

		err = os.Rename(path, backupPath)
		if err != nil {
			return fmt.Errorf("rename existing config file to backup: %w", err)
		}

		configExists = false
	}

	// Write the default config file.
	if !configExists {
		slog.Info("write default configuration",
			slog.String("path", path),
		)

		err = os.WriteFile(path, defaultConfigYAML, 0o600)
		if err != nil {
			return fmt.Errorf("write config file: %w", err)
		}
	} else {
		slog.Debug("configuration file already exists, skipping write",
			slog.String("path", path),
		)
	}

	// Write the JSON schema file alongside the config file.
	schemaPath := filepath.Join(filepath.Dir(path), "config.v1beta1.json")
	slog.Debug("write JSON schema",
		slog.String("path", schemaPath),
	)

	err = os.WriteFile(schemaPath, schemaJSON, 0o600)
	if err != nil {
		return fmt.Errorf("write schema file: %w", err)
	}

	return nil
}

func (c *Config) MarshalYAML() ([]byte, error) {
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b,
		yaml.Indent(2),
		yaml.IndentSequence(true),
	)
	err := enc.Encode(*c)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return b.Bytes(), nil
}

func GetPath() string {
	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		return filepath.Join(xdgHome, "kat", "config.yaml")
	}

	usrHome, err := os.UserHomeDir()
	if err == nil && usrHome != "" {
		return filepath.Join(usrHome, ".config", "kat", "config.yaml")
	}

	tmpConfig := filepath.Join(os.TempDir(), "kat", "config.yaml")

	slog.Warn("could not determine user config directory, using temp path for config",
		slog.String("path", tmpConfig),
		slog.Any("error", fmt.Errorf("$XDG_CONFIG_HOME is unset, fall back to home directory: %w", err)),
	)

	return tmpConfig
}
