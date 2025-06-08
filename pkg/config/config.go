package config

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	yaml "sigs.k8s.io/yaml/goyaml.v3"

	"github.com/MacroPower/kat/pkg/kube"
	uiconfig "github.com/MacroPower/kat/pkg/ui/config"
)

type Config struct {
	Kube *kube.Config     `embed:"" json:"kube" prefix:"kube-" yaml:"kube"`
	UI   *uiconfig.Config `embed:"" json:"ui"   prefix:"ui-"   yaml:"ui"`
}

func NewConfig() *Config {
	return &Config{
		UI:   &uiconfig.DefaultConfig,
		Kube: &kube.DefaultConfig,
	}
}

func (c *Config) EnsureDefaults() {
	if c.UI == nil {
		c.UI = &uiconfig.DefaultConfig
	} else {
		c.UI.EnsureDefaults()
	}

	if c.Kube == nil {
		c.Kube = &kube.DefaultConfig
	} else {
		c.Kube.EnsureDefaults()
	}
}

func (c *Config) Load(path string) error {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && !pathInfo.Mode().IsRegular() {
			return fmt.Errorf("%s: unknown file state", path)
		}
		if pathInfo.IsDir() {
			return fmt.Errorf("%s: path is a directory", path)
		}
	}
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: Potential file inclusion via variable.
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	return nil
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
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)

	if err := enc.Encode(c); err != nil {
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
