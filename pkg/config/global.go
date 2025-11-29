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

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../internal/schemagen/main.go -o config.v1beta1.json

var (
	//go:embed config.yaml
	defaultConfigYAML []byte

	//go:embed config.v1beta1.json
	schemaJSON []byte

	// ValidKinds contains the valid kind values for global configurations.
	ValidKinds = []string{
		"Configuration",
	}

	// DefaultValidator validates global configuration against the JSON schema.
	DefaultValidator = yaml.MustNewValidator("/config.v1beta1.json", schemaJSON)
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

// NewConfig creates a new global [Config] with default values.
func NewConfig() *Config {
	c := &Config{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "Configuration",
	}
	c.EnsureDefaults()

	return c
}

// EnsureDefaults initializes nil fields to their default values.
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
	extendSchemaWithEnums(jss, ValidAPIVersions, ValidKinds)
}

// MarshalYAML serializes the config to YAML.
func (c Config) MarshalYAML() ([]byte, error) {
	type alias Config

	b := &bytes.Buffer{}

	enc := yaml.NewEncoder(b)
	err := enc.Encode(alias(c))
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	defer func() {
		err := enc.Close()
		if err != nil {
			slog.Error("failed to close YAML encoder", slog.Any("error", err))
		}
	}()

	return b.Bytes(), nil
}

// Write writes the config to the specified path if it doesn't already exist.
func (c Config) Write(path string) error {
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

// ConfigLoader loads and validates global configuration files.
type ConfigLoader struct {
	*baseLoader
	policy     *Policy // Policy for trust decisions.
	policyPath string  // Path to the policy file (for persisting trust).
}

// NewConfigLoaderFromBytes creates a [ConfigLoader] from byte data.
func NewConfigLoaderFromBytes(data []byte, opts ...ConfigLoaderOpt) *ConfigLoader {
	return &ConfigLoader{baseLoader: newBaseLoader(data, DefaultValidator, opts...)}
}

// NewConfigLoaderFromFile creates a [ConfigLoader] from a file path.
func NewConfigLoaderFromFile(path string, opts ...ConfigLoaderOpt) (*ConfigLoader, error) {
	data, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Load policy file for trust decisions.
	policyPath := GetPolicyPath()

	var policy *Policy

	pl, plErr := NewPolicyLoaderFromFile(policyPath, opts...)
	if plErr == nil {
		validateErr := pl.Validate()
		if validateErr == nil {
			policy, _ = pl.Load() //nolint:errcheck // We use nil check below.
		}
	}

	if policy == nil {
		policy = NewPolicy()
	}

	return &ConfigLoader{
		baseLoader: newBaseLoader(data, DefaultValidator, opts...),
		policy:     policy,
		policyPath: policyPath,
	}, nil
}

// Load parses and returns the Config.
func (cl *ConfigLoader) Load() (*Config, error) {
	c := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(cl.data))

	err := dec.Decode(c)
	if err != nil {
		return nil, cl.yamlError.Wrap(err)
	}

	c.EnsureDefaults()

	err = c.Command.Validate()
	if err != nil {
		return nil, cl.yamlError.Wrap(err)
	}

	return c, nil
}

// LoadWithProjectConfig loads the global config and merges any project config found.
// It handles trust checking/prompting based on the provided options.
func (cl *ConfigLoader) LoadWithProjectConfig(tp TrustPrompter, tm TrustMode, targetPath string) (*Config, error) {
	cfg, err := cl.Load()
	if err != nil {
		return nil, err
	}

	projectCfgPath, err := FindProjectConfig(targetPath)
	if err != nil {
		return nil, fmt.Errorf("find project config: %w", err)
	}

	if projectCfgPath == "" {
		return cfg, nil
	}

	projectDir := filepath.Dir(projectCfgPath)

	trusted, err := cl.ensureProjectTrusted(projectDir, projectCfgPath, tp, tm)
	if err != nil {
		return nil, err
	}

	if !trusted {
		slog.Warn("skipping untrusted project configuration", slog.String("path", projectCfgPath))

		return cfg, nil
	}

	pcl, err := NewProjectConfigLoaderFromFile(projectCfgPath, WithThemeFromData())
	if err != nil {
		return nil, fmt.Errorf("create project loader: %w", err)
	}

	err = pcl.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate project config %q: %w", projectCfgPath, err)
	}

	projectCfg, err := pcl.Load()
	if err != nil {
		return nil, fmt.Errorf("load project config %q: %w", projectCfgPath, err)
	}

	cfg.Command.Merge(projectCfg.Command)

	slog.Debug("loaded project configuration", slog.String("path", projectCfgPath))

	return cfg, nil
}

// ensureProjectTrusted checks if a project is trusted and prompts the user if not.
// Returns true if the project is trusted (or user allowed it), false if skipped.
func (cl *ConfigLoader) ensureProjectTrusted(
	projectDir, projectCfgPath string,
	tp TrustPrompter,
	tm TrustMode,
) (bool, error) {
	switch tm {
	case TrustModeSkip:
		slog.Info("skipping project config (--no-trust)", slog.String("path", projectCfgPath))

		return false, nil

	case TrustModeAllow:
		slog.Info("trusting project config (--trust)", slog.String("path", projectCfgPath))

		err := cl.policy.TrustProject(projectDir, cl.policyPath)
		if err != nil {
			slog.Warn("could not save trusted project", slog.Any("err", err))
		}

		return true, nil

	case TrustModePrompt:
		// Check if already trusted in policy.
		if cl.policy.IsTrusted(projectDir) {
			return true, nil
		}

		if tp == nil {
			slog.Warn(
				"skipping untrusted project config (no prompter)",
				slog.String("path", projectCfgPath),
			)

			return false, nil
		}

		decision, err := tp.Trust(projectDir, projectCfgPath)
		if errors.Is(err, ErrNotInteractive) {
			slog.Warn(
				"skipping untrusted project config (non-interactive)",
				slog.String("path", projectCfgPath),
				slog.String(
					"hint",
					"run kat interactively to trust this project, or use --trust/--no-trust flags",
				),
			)

			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("prompt: %w", err)
		}

		if decision == TrustSkip {
			return false, nil
		}

		err = cl.policy.TrustProject(projectDir, cl.policyPath)
		if err != nil {
			slog.Warn("could not save trusted project", slog.Any("err", err))
		}

		return true, nil

	default:
		return false, fmt.Errorf("unknown trust mode: %d", tm)
	}
}

// WriteDefaultConfig writes the embedded default config.yaml to the specified path.
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
		backupFile := fmt.Sprintf("%s.%d.old", filepath.Base(path), time.Now().UnixNano())
		backupPath := filepath.Join(filepath.Dir(path), backupFile)
		slog.Info("backing up existing config file", slog.String("path", backupPath))

		err = os.Rename(path, backupPath)
		if err != nil {
			return fmt.Errorf("rename existing config file to backup: %w", err)
		}

		configExists = false
	}

	if !configExists {
		slog.Info("write default configuration", slog.String("path", path))

		err = os.WriteFile(path, defaultConfigYAML, 0o600)
		if err != nil {
			return fmt.Errorf("write config file: %w", err)
		}
	} else {
		slog.Debug("configuration file already exists, skipping write", slog.String("path", path))
	}

	// Also write default policy file if it doesn't exist.
	policyPath := GetPolicyPath()

	err = WriteDefaultPolicy(policyPath, false)
	if err != nil {
		slog.Warn("could not write default policy file", slog.Any("err", err))
	}

	return nil
}

// GetPath returns the path to the global configuration file.
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
