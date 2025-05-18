package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

var yamlExtensions = []string{
	".yaml", ".yml",
}

// IsYAMLFile returns whether the filename has a YAML extension.
func IsYAMLFile(filename string) bool {
	ext := filepath.Ext(filename)

	if ext == "" {
		// By default, assume it's a YAML file.
		return true
	}

	for _, v := range yamlExtensions {
		if strings.EqualFold(ext, v) {
			return true
		}
	}

	// Has an extension but not yaml
	// so assume this is a code file.
	return false
}

// GlamourStyle returns ansi.StyleConfig based on the given style.
func GlamourStyle(style string, isCode bool) (ansi.StyleConfig, error) {
	if !isCode {
		if style == styles.AutoStyle {
			return getDefaultStyle(style)
		}

		return withStylePath(style)
	}

	// If we are rendering a pure code block, we need to modify the style to
	// remove the indentation.

	var styleConfig ansi.StyleConfig

	switch style {
	case styles.AutoStyle:
		if lipgloss.HasDarkBackground() {
			styleConfig = styles.DarkStyleConfig
		} else {
			styleConfig = styles.LightStyleConfig
		}
	case styles.DarkStyle:
		styleConfig = styles.DarkStyleConfig
	case styles.LightStyle:
		styleConfig = styles.LightStyleConfig
	case styles.PinkStyle:
		styleConfig = styles.PinkStyleConfig
	case styles.NoTTYStyle:
		styleConfig = styles.NoTTYStyleConfig
	case styles.DraculaStyle:
		styleConfig = styles.DraculaStyleConfig
	case styles.TokyoNightStyle:
		styleConfig = styles.DraculaStyleConfig
	default:
		return withStylesFromJSONFile(style)
	}

	var margin uint
	styleConfig.CodeBlock.Margin = &margin

	return styleConfig, nil
}

// withStylesFromJSONFile sets a TermRenderer's styles from a JSON file.
func withStylesFromJSONFile(filename string) (ansi.StyleConfig, error) {
	var styleConfig ansi.StyleConfig

	jsonBytes, err := os.ReadFile(filename) //nolint:gosec // G304: Potential file inclusion via variable.
	if err != nil {
		return styleConfig, fmt.Errorf("glamour: error reading file: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &styleConfig); err != nil {
		return styleConfig, fmt.Errorf("glamour: error parsing file: %w", err)
	}

	return styleConfig, nil
}

func getDefaultStyle(style string) (ansi.StyleConfig, error) {
	if style == styles.AutoStyle {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return styles.NoTTYStyleConfig, nil
		}
		if termenv.HasDarkBackground() {
			return styles.DarkStyleConfig, nil
		}

		return styles.LightStyleConfig, nil
	}

	ds, ok := styles.DefaultStyles[style]
	if !ok {
		return ansi.StyleConfig{}, fmt.Errorf("%s: style not found", style)
	}

	return *ds, nil
}

func withStylePath(stylePath string) (ansi.StyleConfig, error) {
	ds, err := getDefaultStyle(stylePath)
	if err != nil {
		return withStylesFromJSONFile(stylePath)
	}

	return ds, nil
}
