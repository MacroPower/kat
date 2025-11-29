package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/macropower/kat/pkg/yaml"
)

// GetConfigPath returns the path to a configuration file in the user's config directory.
// It checks $XDG_CONFIG_HOME first, then falls back to ~/.config, and finally to a temp directory.
func GetConfigPath(filename string) string {
	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		return filepath.Join(xdgHome, "kat", filename)
	}

	usrHome, err := os.UserHomeDir()
	if err == nil && usrHome != "" {
		return filepath.Join(usrHome, ".config", "kat", filename)
	}

	tmpPath := filepath.Join(os.TempDir(), "kat", filename)

	slog.Warn("could not determine user config directory, using temp path",
		slog.String("path", tmpPath),
		slog.Any("error", fmt.Errorf("$XDG_CONFIG_HOME is unset, fall back to home directory: %w", err)),
	)

	return tmpPath
}

// ReadFile reads a file from disk with proper error handling.
func ReadFile(path string) ([]byte, error) {
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

// MarshalYAML serializes an object to YAML bytes.
func MarshalYAML(obj any) ([]byte, error) {
	b := &bytes.Buffer{}

	enc := yaml.NewEncoder(b)
	err := enc.Encode(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	defer func() {
		err := enc.Close()
		if err != nil {
			slog.Error("close YAML encoder", slog.Any("error", err))
		}
	}()

	return b.Bytes(), nil
}

// WriteIfNotExists writes data to a path if the file doesn't already exist.
func WriteIfNotExists(path string, data []byte) error {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.Mode().IsRegular() {
			return nil // File already exists.
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

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// FindConfigFile searches for a config file starting from targetPath
// and walking up the directory tree until the filesystem root.
// It checks for all provided fileNames in each directory.
// Returns the path to the config file if found, or empty string if not found.
func FindConfigFile(targetPath string, fileNames []string) (string, error) {
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

	// Walk up the directory tree looking for config files.
	for {
		for _, fileName := range fileNames {
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

// WriteDefaultFile writes default content to a path.
// Using `force` will back up and replace any existing files.
func WriteDefaultFile(path string, defaultData []byte, force bool, kind string) error {
	fileExists := false

	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		switch {
		case err == nil && pathInfo.Mode().IsRegular():
			fileExists = true
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

	if fileExists && force {
		backupFile := fmt.Sprintf("%s.%d.old", filepath.Base(path), time.Now().UnixNano())
		backupPath := filepath.Join(filepath.Dir(path), backupFile)
		slog.Info("backing up existing file",
			slog.String("type", kind),
			slog.String("path", backupPath),
		)

		err = os.Rename(path, backupPath)
		if err != nil {
			return fmt.Errorf("rename existing %s file to backup: %w", kind, err)
		}

		fileExists = false
	}

	if !fileExists {
		slog.Info("write default file",
			slog.String("type", kind),
			slog.String("path", path),
		)

		err = os.WriteFile(path, defaultData, 0o600)
		if err != nil {
			return fmt.Errorf("write %s file: %w", kind, err)
		}
	} else {
		slog.Debug("file already exists, skipping write",
			slog.String("type", kind),
			slog.String("path", path),
		)
	}

	return nil
}
