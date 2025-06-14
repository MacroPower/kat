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
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	glamouransi "github.com/charmbracelet/glamour/ansi"
)

const (
	lineNumberWidth = 4

	wrapOnCharacters = " /-"
)

var (
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
	if gr.model.cm.Config.GlamourDisabled {
		return yaml, nil
	}

	content, err := gr.executeRendering(yaml)
	if err != nil {
		return "", err
	}

	return gr.postProcessContent(content), nil
}

// executeRendering performs the actual glamour rendering.
func (gr *GlamourRenderer) executeRendering(yaml string) (string, error) {
	style, err := glamourStyle(gr.model.cm.Config.GlamourStyle)
	if err != nil {
		return "", err
	}

	r := glamouransi.CodeBlockElement{
		Code:     yaml,
		Language: "yaml",
	}

	buf := &bytes.Buffer{}
	err = r.Render(buf, glamouransi.NewRenderContext(glamouransi.Options{
		Styles:           style,
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
			result.WriteString(gr.formatLine(line))
		}

		// Don't add an artificial newline after the last split.
		if i+1 < len(lines) {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

func (gr *GlamourRenderer) formatLine(line string) string {
	viewMaxWidth := max(0, gr.model.viewport.Width)
	glamourMaxWidth := min(viewMaxWidth, max(0, gr.model.cm.Config.GlamourMaxWidth))

	trunc := lipgloss.NewStyle().MaxWidth(viewMaxWidth).Render
	lines := cellbuf.Wrap(line, glamourMaxWidth, wrapOnCharacters)

	return trunc(lines)
}

// formatLineWithNumber formats a line with line number and truncation.
func (gr *GlamourRenderer) formatLineWithNumber(line string, lineNum int) string {
	viewMaxWidth := max(0, gr.model.viewport.Width-lineNumberWidth)
	glamourMaxWidth := min(viewMaxWidth, max(0, gr.model.cm.Config.GlamourMaxWidth-lineNumberWidth))

	trunc := lipgloss.NewStyle().MaxWidth(viewMaxWidth).Render
	lineNumberText := fmt.Sprintf("%"+strconv.Itoa(lineNumberWidth)+"d", lineNum)

	lines := cellbuf.Wrap(line, glamourMaxWidth, wrapOnCharacters)

	fmtLines := []string{}
	for i, ln := range strings.Split(lines, "\n") {
		if i == 0 {
			// Add line number only to the first line.
			ln = lineNumberStyle(lineNumberText) + trunc(ln)
		} else {
			// For subsequent lines, just add spaces for alignment.
			ln = strings.Repeat(" ", max(0, lineNumberWidth-2)) + lineNumberStyle(" -  ") + trunc(ln)
		}
		fmtLines = append(fmtLines, ln)
	}

	return strings.Join(fmtLines, "\n")
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
