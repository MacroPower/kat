package pager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	glamouransi "github.com/charmbracelet/glamour/ansi"
)

const lineNumberWidth = 4

var (
	GlamourDisabled = false

	lineNumberFg    = lipgloss.AdaptiveColor{Light: "#656565", Dark: "#7D7D7D"}
	lineNumberStyle = lipgloss.NewStyle().
			Foreground(lineNumberFg).
			Render
)

// GlamourRenderer handles rendering content with glamour styling.
type GlamourRenderer struct {
	model PagerModel
}

// NewGlamourRenderer creates a new GlamourRenderer.
func NewGlamourRenderer(model PagerModel) *GlamourRenderer {
	return &GlamourRenderer{model: model}
}

// RenderContent renders YAML content with glamour styling.
func (gr *GlamourRenderer) RenderContent(yaml string) (string, error) {
	if GlamourDisabled {
		return yaml, nil
	}

	renderConfig, err := gr.buildRenderConfig()
	if err != nil {
		return "", err
	}

	content, err := gr.executeRendering(yaml, renderConfig)
	if err != nil {
		return "", err
	}

	return gr.postProcessContent(content), nil
}

// buildRenderConfig creates the rendering configuration.
func (gr *GlamourRenderer) buildRenderConfig() (*glamourRenderConfig, error) {
	const width = 0

	style, err := glamourStyle(gr.model.cm.Config.GlamourStyle)
	if err != nil {
		return nil, err
	}

	return &glamourRenderConfig{
		width: width,
		style: style,
	}, nil
}

// executeRendering performs the actual glamour rendering.
func (gr *GlamourRenderer) executeRendering(yaml string, config *glamourRenderConfig) (string, error) {
	r := glamouransi.CodeBlockElement{
		Code:     yaml,
		Language: "yaml",
	}

	buf := &bytes.Buffer{}
	err := r.Render(buf, glamouransi.NewRenderContext(glamouransi.Options{
		WordWrap:         config.width,
		Styles:           config.style,
		PreserveNewLines: true,
	}))
	if err != nil {
		return "", fmt.Errorf("error creating glamour renderer: %w", err)
	}

	return buf.String(), nil
}

// postProcessContent handles post-processing of rendered content.
func (gr *GlamourRenderer) postProcessContent(content string) string {
	content = strings.TrimSpace(content)

	// Trim lines and add line numbers if needed.
	lines := strings.Split(content, "\n")
	var result strings.Builder

	for i, line := range lines {
		if !gr.model.cm.Config.LineNumbersDisabled {
			result.WriteString(gr.formatLineWithNumber(line, i+1))
		} else {
			result.WriteString(line)
		}

		// Don't add an artificial newline after the last split.
		if i+1 < len(lines) {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// formatLineWithNumber formats a line with line number and truncation.
func (gr *GlamourRenderer) formatLineWithNumber(line string, lineNum int) string {
	trunc := lipgloss.NewStyle().MaxWidth(gr.model.viewport.Width - lineNumberWidth).Render
	lineNumberText := fmt.Sprintf("%"+strconv.Itoa(lineNumberWidth)+"d", lineNum)

	return lineNumberStyle(lineNumberText) + trunc(line)
}

// glamourRenderConfig holds configuration for glamour rendering.
type glamourRenderConfig struct {
	style glamouransi.StyleConfig
	width int
}

// glamourStyle returns ansi.StyleConfig based on the given style.
func glamourStyle(style string) (glamouransi.StyleConfig, error) {
	if style == styles.AutoStyle {
		return getDefaultStyle(style)
	}

	return withStylePath(style)
}

// withStylesFromJSONFile sets a TermRenderer's styles from a JSON file.
func withStylesFromJSONFile(filename string) (glamouransi.StyleConfig, error) {
	var styleConfig glamouransi.StyleConfig

	jsonBytes, err := os.ReadFile(filename) //nolint:gosec // G304: Potential file inclusion via variable.
	if err != nil {
		return styleConfig, fmt.Errorf("glamour: error reading file: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &styleConfig); err != nil {
		return styleConfig, fmt.Errorf("glamour: error parsing file: %w", err)
	}

	return styleConfig, nil
}

func getDefaultStyle(style string) (glamouransi.StyleConfig, error) {
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
		return glamouransi.StyleConfig{}, fmt.Errorf("%s: style not found", style)
	}

	return *ds, nil
}

func withStylePath(stylePath string) (glamouransi.StyleConfig, error) {
	ds, err := getDefaultStyle(stylePath)
	if err != nil {
		return withStylesFromJSONFile(stylePath)
	}

	return ds, nil
}
