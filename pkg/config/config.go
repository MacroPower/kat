package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui"
	"gopkg.in/yaml.v3"
)

type Config struct {
	UI   ui.Config   `embed:"" prefix:"ui-"`
	Kube kube.Config `embed:"" prefix:"kube-"`
}

func NewConfig() *Config {
	return &Config{
		UI:   ui.DefaultConfig,
		Kube: kube.DefaultConfig,
	}
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

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
