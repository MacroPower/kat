package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-playground/validator/v10"

	yaml "github.com/goccy/go-yaml"

	"github.com/MacroPower/kat/pkg/kube"
	ui "github.com/MacroPower/kat/pkg/ui/config"
)

var (
	ValidAPIVersions = []string{
		"kat.jacobcolvin.com/v1beta1",
	}
	ValidKinds = []string{
		"Configuration",
	}
)

type Config struct {
	Kube       *kube.Config `yaml:",inline"`
	UI         *ui.Config   `yaml:",inline"`
	APIVersion string       `validate:"required" yaml:"apiVersion"`
	Kind       string       `validate:"required" yaml:"kind"`
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

	if c.Kube == nil {
		c.Kube = kube.DefaultConfig
	} else {
		c.Kube.EnsureDefaults()
	}
}

func (c *Config) Validate() error {
	if !slices.Contains(ValidAPIVersions, c.APIVersion) {
		return fmt.Errorf("unsupported apiVersion %q, expected one of [%s]", c.APIVersion, strings.Join(ValidAPIVersions, ", "))
	}
	if !slices.Contains(ValidKinds, c.Kind) {
		return fmt.Errorf("unsupported kind %q, expected one of [%s]", c.Kind, strings.Join(ValidKinds, ", "))
	}

	return nil
}

func ReadConfig(path string) ([]byte, error) {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && !pathInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: unknown file state", path)
		}
		if pathInfo.IsDir() {
			return nil, fmt.Errorf("%s: path is a directory", path)
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
	c := &Config{}
	validate := validator.New(validator.WithRequiredStructEnabled())
	dec := yaml.NewDecoder(bytes.NewReader(data),
		yaml.Validator(validate),
		yaml.AllowDuplicateMapKey(),
	)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode yaml config: %w", err)
	}

	c.EnsureDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := c.Kube.Validate(); err != nil {
		source, srcErr := err.Path.AnnotateSource(data, true)
		if srcErr != nil {
			panic(srcErr)
		}

		return nil, fmt.Errorf("validate config: %w\n%s", err, source)
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

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	b, err := c.MarshalYAML()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (c *Config) MarshalYAML() ([]byte, error) {
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b,
		yaml.Indent(2),
		yaml.IndentSequence(true),
	)
	if err := enc.Encode(*c); err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return b.Bytes(), nil
}

func GetPath() string {
	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		return filepath.Join(xdgHome, "kat", "config.yaml")
	}

	if usr, err := user.Current(); err != nil {
		return filepath.Join(usr.HomeDir, ".config", "kat", "config.yaml")
	}

	return filepath.Join(os.TempDir(), "kat", "config.yaml")
}
