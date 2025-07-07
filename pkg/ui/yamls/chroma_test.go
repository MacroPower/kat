package yamls_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/themes"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// testTheme creates a basic theme for testing.
func testTheme() *themes.Theme {
	return &themes.Theme{
		SelectedStyle:   lipgloss.NewStyle().Background(lipgloss.Color("12")).Underline(true).Bold(true),
		LineNumberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		ChromaStyle:     styles.Get("github"),
	}
}

// verifyContentPresence checks that expected content is present in the rendered result.
func verifyContentPresence(t *testing.T, plainResult, originalYaml string, width int) {
	t.Helper()

	trimmedYaml := strings.TrimSpace(originalYaml)
	if trimmedYaml == "" {
		return
	}

	lines := strings.Split(trimmedYaml, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if width < 20 {
			verifyWrappedContent(t, plainResult, line)
		} else {
			assert.Contains(t, plainResult, line, "Should contain line: %s", line)
		}
	}
}

// verifyWrappedContent handles content verification for narrow widths where wrapping occurs.
func verifyWrappedContent(t *testing.T, plainResult, line string) {
	t.Helper()

	if !strings.Contains(line, ":") {
		assert.Contains(t, plainResult, line, "Should contain line: %s", line)

		return
	}

	// Handle key-value pairs that may be wrapped.
	parts := strings.Split(line, ":")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if len(part) > 3 {
			// Check for beginning of the word since it may be wrapped.
			prefix := part[:3]
			assert.Contains(t, plainResult, prefix, "Should contain prefix: %s", prefix)
		} else {
			assert.Contains(t, plainResult, part, "Should contain part: %s", part)
		}
	}
}

func TestNewChromaRenderer(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		theme               *themes.Theme
		lineNumbersDisabled bool
	}{
		"with line numbers enabled": {
			theme:               testTheme(),
			lineNumbersDisabled: false,
		},
		"with line numbers disabled": {
			theme:               testTheme(),
			lineNumbersDisabled: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(tc.theme, tc.lineNumbersDisabled)
			assert.NotNil(t, renderer)

			// Test that the renderer can render basic content.
			result, err := renderer.RenderContent("key: value", 80)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
		})
	}
}

func TestChromaRenderer_SetAndGetMethods(t *testing.T) {
	t.Parallel()

	renderer := yamls.NewChromaRenderer(testTheme(), true)

	// Test SetSearchTerm and GetMatches.
	assert.Empty(t, renderer.GetMatches())

	renderer.SetSearchTerm("test")
	assert.Empty(t, renderer.GetMatches()) // Should be empty until RenderContent is called.

	// Test SetFormatter.
	renderer.SetFormatter("terminal16m")

	// Render content to populate matches.
	_, err := renderer.RenderContent("test: value test", 80)
	require.NoError(t, err)

	matches := renderer.GetMatches()
	assert.NotEmpty(t, matches)

	// Test clearing search term.
	renderer.SetSearchTerm("")
	assert.Empty(t, renderer.GetMatches())
}

func TestChromaRenderer_RenderContent_Basic(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	testCases := map[string]struct {
		yaml    string
		width   int
		wantErr bool
	}{
		"simple yaml": {
			yaml:    "key: value",
			width:   80,
			wantErr: false,
		},
		"multi-line yaml": {
			yaml:    "key1: value1\nkey2: value2\nkey3: value3",
			width:   80,
			wantErr: false,
		},
		"complex yaml": {
			yaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
data:
  config.yaml: |
    setting1: value1
    setting2: value2
`,
			width:   80,
			wantErr: false,
		},
		"empty yaml": {
			yaml:    "",
			width:   80,
			wantErr: false,
		},
		"yaml with special characters": {
			yaml:    "key: \"value with spaces and symbols: !@#$%^&*()\"",
			width:   80,
			wantErr: false,
		},
		"very narrow width": {
			yaml:    "key: value",
			width:   5,
			wantErr: false,
		},
		"zero width": {
			yaml:    "key: value",
			width:   0,
			wantErr: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), false)
			renderer.SetFormatter("terminal16m")

			result, err := renderer.RenderContent(tc.yaml, tc.width)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify that the plain text content matches after stripping ANSI.
				plainResult := ansi.Strip(result)
				if tc.yaml != "" && !tc.wantErr {
					verifyContentPresence(t, plainResult, tc.yaml, tc.width)
				}
			}
		})
	}
}

func TestChromaRenderer_SearchHighlighting(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	testCases := map[string]struct {
		yaml          string
		searchTerm    string
		description   string
		expectMatches int
	}{
		"single character exact match": {
			yaml:          "text: hello world",
			searchTerm:    "o",
			expectMatches: 2,
			description:   "Should find both 'o' characters in 'hello world'",
		},
		"multi-character fuzzy match": {
			yaml:          "name: hello-world",
			searchTerm:    "hello",
			expectMatches: 1,
			description:   "Should find 'hello' as a group",
		},
		"case insensitive search": {
			yaml:          "Name: VALUE",
			searchTerm:    "name",
			expectMatches: 1,
			description:   "Should find 'Name' when searching for 'name'",
		},
		"no matches": {
			yaml:          "key: value",
			searchTerm:    "xyz",
			expectMatches: 0,
			description:   "Should find no matches for non-existent term",
		},
		"overlapping matches": {
			yaml:          "aaaaaa",
			searchTerm:    "a",
			expectMatches: 1,
			description:   "Should group consecutive matches",
		},
		"special characters": {
			yaml:          "key: value-test_123",
			searchTerm:    "-",
			expectMatches: 1,
			description:   "Should find special characters",
		},
		"unicode characters": {
			yaml:          "name: café",
			searchTerm:    "café",
			expectMatches: 1,
			description:   "Should handle unicode characters",
		},
		"empty search term": {
			yaml:          "key: value",
			searchTerm:    "",
			expectMatches: 0,
			description:   "Should handle empty search term",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), true)
			renderer.SetFormatter("terminal16m")
			renderer.SetSearchTerm(tc.searchTerm)

			result, err := renderer.RenderContent(tc.yaml, 80)
			require.NoError(t, err, tc.description)

			matches := renderer.GetMatches()
			assert.Len(t, matches, tc.expectMatches, tc.description)

			// Verify that highlights are applied when matches exist.
			if tc.expectMatches > 0 {
				assert.Contains(t, result, "\x1b[", "Expected ANSI sequences for highlighting")
			}

			// Verify that plain text is preserved.
			plainResult := ansi.Strip(result)
			assert.Equal(t, tc.yaml, plainResult, "Plain text should be preserved")
		})
	}
}

func TestChromaRenderer_LineNumbers(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	testCases := map[string]struct {
		yaml                 string
		lineNumbersDisabled  bool
		shouldContainLineNum bool
	}{
		"with line numbers": {
			lineNumbersDisabled:  false,
			yaml:                 "line1: value1\nline2: value2\nline3: value3",
			shouldContainLineNum: true,
		},
		"without line numbers": {
			lineNumbersDisabled:  true,
			yaml:                 "line1: value1\nline2: value2\nline3: value3",
			shouldContainLineNum: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), tc.lineNumbersDisabled)
			renderer.SetFormatter("terminal16m")

			result, err := renderer.RenderContent(tc.yaml, 80)
			require.NoError(t, err)

			if tc.shouldContainLineNum {
				// Should contain line numbers like "   1  ".
				assert.Regexp(t, `\s+\d+\s+`, result, "Should contain line numbers")
			} else {
				// Should not contain line number patterns.
				assert.NotRegexp(t, `\s+\d+\s+`, result, "Should not contain line numbers")
			}
		})
	}
}

func TestChromaRenderer_MatchPositioning(t *testing.T) {
	t.Parallel()

	renderer := yamls.NewChromaRenderer(testTheme(), true)
	renderer.SetFormatter("terminal16m")

	// Test specific match positioning.
	yaml := "text: hello world"
	renderer.SetSearchTerm("o")

	_, err := renderer.RenderContent(yaml, 80)
	require.NoError(t, err)

	matches := renderer.GetMatches()
	require.Len(t, matches, 2, "Should find exactly 2 matches for 'o'")

	// Verify positions of 'o' in "hell[o] w[o]rld".
	expectedPositions := []struct {
		line  int
		start int
		end   int
	}{
		{line: 0, start: 10, end: 11}, // First 'o' in "hello"
		{line: 0, start: 13, end: 14}, // Second 'o' in "world"
	}

	for i, expected := range expectedPositions {
		assert.Equal(t, expected.line, matches[i].Line, "Match %d line position", i)
		assert.Equal(t, expected.start, matches[i].Start, "Match %d start position", i)
		assert.Equal(t, expected.end, matches[i].End, "Match %d end position", i)
		assert.Equal(t, 1, matches[i].Length, "Match %d length", i)
	}
}

func TestChromaRenderer_ConsecutiveMatches(t *testing.T) {
	t.Parallel()

	renderer := yamls.NewChromaRenderer(testTheme(), true)
	renderer.SetFormatter("terminal16m")

	// Test consecutive character matching and grouping.
	yaml := "hello"
	renderer.SetSearchTerm("l")

	_, err := renderer.RenderContent(yaml, 80)
	require.NoError(t, err)

	matches := renderer.GetMatches()
	require.Len(t, matches, 1, "Should group consecutive 'l' characters into one match")

	match := matches[0]
	assert.Equal(t, 0, match.Line)
	assert.Equal(t, 2, match.Start)  // First 'l' in "hello"
	assert.Equal(t, 4, match.End)    // After last 'l' in "hello"
	assert.Equal(t, 2, match.Length) // Two consecutive 'l' characters
}

func TestChromaRenderer_MultiLineSearch(t *testing.T) {
	t.Parallel()

	renderer := yamls.NewChromaRenderer(testTheme(), true)
	renderer.SetFormatter("terminal16m")

	yaml := `line1: test
line2: test
line3: other`
	renderer.SetSearchTerm("test")

	_, err := renderer.RenderContent(yaml, 80)
	require.NoError(t, err)

	matches := renderer.GetMatches()
	assert.Len(t, matches, 2, "Should find 'test' on two different lines")

	// Verify matches are on different lines.
	assert.Equal(t, 0, matches[0].Line, "First match should be on line 0")
	assert.Equal(t, 1, matches[1].Line, "Second match should be on line 1")
}

func TestChromaRenderer_ANSIHandling(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	renderer := yamls.NewChromaRenderer(testTheme(), true)
	renderer.SetFormatter("terminal16m")

	text := "key: value"
	renderer.SetSearchTerm("value")

	result, err := renderer.RenderContent(text, 80)
	require.NoError(t, err)

	// Verify that the result contains both chroma styling and search highlighting.
	assert.Contains(t, result, "\x1b[", "Should contain ANSI escape sequences")

	// Verify that when we strip ANSI, we get the original text.
	plainResult := ansi.Strip(result)
	assert.Equal(t, text, plainResult, "Plain text should match original after stripping ANSI")

	// Verify that highlighting is applied.
	matches := renderer.GetMatches()
	assert.Len(t, matches, 1, "Should find one match for 'value'")
}

func TestChromaRenderer_EdgeCases(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		yaml       string
		searchTerm string
		width      int
		expectErr  bool
	}{
		"very long line": {
			yaml:       strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100),
			searchTerm: "z",
			width:      80,
			expectErr:  false,
		},
		"unicode in yaml": {
			yaml:       "name: 你好世界",
			searchTerm: "好",
			width:      80,
			expectErr:  false,
		},
		"emoji in yaml": {
			yaml:       "status: 🚀 ready",
			searchTerm: "🚀",
			width:      80,
			expectErr:  false,
		},
		"tab characters": {
			yaml:       "key:\tvalue",
			searchTerm: "value",
			width:      80,
			expectErr:  false,
		},
		"mixed whitespace": {
			yaml:       "key:   \t  value",
			searchTerm: "value",
			width:      80,
			expectErr:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), true)
			renderer.SetFormatter("terminal16m")
			renderer.SetSearchTerm(tc.searchTerm)

			_, err := renderer.RenderContent(tc.yaml, tc.width)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				if tc.searchTerm != "" {
					matches := renderer.GetMatches()
					// Don't assert specific count since it depends on content,
					// just verify the method doesn't crash.
					t.Logf("Found %d matches for '%s' in '%s'", len(matches), tc.searchTerm, tc.yaml)
				}
			}
		})
	}
}

func TestChromaRenderer_DifferentFormatters(t *testing.T) {
	t.Parallel()

	formatters := []string{
		"noop",
		"terminal8",
		"terminal256",
		"terminal16m",
	}

	yaml := "key: value"

	for _, formatter := range formatters {
		t.Run(formatter, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), true)
			renderer.SetFormatter(formatter)

			result, err := renderer.RenderContent(yaml, 80)
			require.NoError(t, err, "Formatter %s should work", formatter)
			assert.NotEmpty(t, result, "Result should not be empty for formatter %s", formatter)
		})
	}
}

func TestChromaRenderer(t *testing.T) {
	t.Parallel()

	// Force lipgloss to use a specific renderer profile.
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a basic theme for testing.
	theme := &themes.Theme{
		SelectedStyle:   lipgloss.NewStyle().Background(lipgloss.Color("12")),
		LineNumberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		ChromaStyle:     styles.Get("github"),
	}

	// Create a ChromaRenderer instance with the theme.
	renderer := yamls.NewChromaRenderer(theme, true)
	renderer.SetFormatter("terminal16m")

	// Test case that reproduces the bug: searching for "o" in "text: hello world"
	yaml := "text: hello world"
	renderer.SetSearchTerm("o")

	result, err := renderer.RenderContent(yaml, 80)
	require.NoError(t, err)

	t.Logf("Original YAML: %q", yaml)
	t.Logf("Rendered result: %q", result)

	// Let's also test the matches
	matches := renderer.GetMatches()
	t.Logf("Found %d matches", len(matches))
	for i, match := range matches {
		t.Logf("Match %d: Line=%d, Start=%d, End=%d, Length=%d",
			i, match.Line, match.Start, match.End, match.Length)
	}

	// Test that we should find both "o" characters at positions 10 and 13
	expectedPositions := []int{10, 13} // "hell[o] w[o]rld"
	assert.Len(t, matches, len(expectedPositions))

	for i, expectedPos := range expectedPositions {
		if i < len(matches) {
			assert.Equal(t, expectedPos, matches[i].Start, "Match %d position", i)
		}
	}
}

func TestChromaRenderer_ErrorScenarios(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		setupFunc   func() *yamls.ChromaRenderer
		yaml        string
		description string
		expectError bool
	}{
		"malformed yaml": {
			setupFunc: func() *yamls.ChromaRenderer {
				return yamls.NewChromaRenderer(testTheme(), true)
			},
			yaml:        "key: [\ninvalid",
			expectError: false, // Chroma should handle malformed YAML gracefully
			description: "Should handle malformed YAML without error",
		},
		"very large content": {
			setupFunc: func() *yamls.ChromaRenderer {
				return yamls.NewChromaRenderer(testTheme(), true)
			},
			yaml:        strings.Repeat("key: value\n", 10000),
			expectError: false,
			description: "Should handle very large content",
		},
		"binary content": {
			setupFunc: func() *yamls.ChromaRenderer {
				return yamls.NewChromaRenderer(testTheme(), true)
			},
			yaml:        string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF}),
			expectError: false,
			description: "Should handle binary content without crashing",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := tc.setupFunc()
			require.NotNil(t, renderer)

			renderer.SetFormatter("terminal16m")

			_, err := renderer.RenderContent(tc.yaml, 80)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

func TestChromaRenderer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Test that the renderer is safe for concurrent read access.
	renderer := yamls.NewChromaRenderer(testTheme(), true)
	renderer.SetFormatter("terminal16m")

	// Prepare the renderer with initial content.
	_, err := renderer.RenderContent("foo: bar", 80)
	require.NoError(t, err)

	// Test concurrent read access to GetMatches.
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			defer func() { done <- true }()

			// These should not panic or race.
			matches := renderer.GetMatches()
			// Don't use assert.NotNil in goroutines - it can cause race conditions.
			// Just check that it doesn't panic.
			_ = matches
		}()
	}

	// Wait for all goroutines to complete.
	for range 10 {
		<-done
	}
}

func TestChromaRenderer_SearchTermNormalization(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		yaml       string
		searchTerm string
		shouldFind bool
	}{
		"accented characters": {
			yaml:       "name: café",
			searchTerm: "cafe", // Without accent
			shouldFind: true,   // Should find due to normalization
		},
		"case difference": {
			yaml:       "Name: VALUE",
			searchTerm: "value",
			shouldFind: true, // Should find due to case insensitivity
		},
		"diacritics": {
			yaml:       "résumé: test",
			searchTerm: "resume",
			shouldFind: true, // Should find due to diacritic removal
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), true)
			renderer.SetFormatter("terminal16m")
			renderer.SetSearchTerm(tc.searchTerm)

			_, err := renderer.RenderContent(tc.yaml, 80)
			require.NoError(t, err)

			matches := renderer.GetMatches()

			if tc.shouldFind {
				assert.NotEmpty(t, matches, "Should find matches for normalized search")
			} else {
				assert.Empty(t, matches, "Should not find matches")
			}
		})
	}
}

func TestChromaRenderer_LineWrapping(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		yaml     string
		expected string
		width    int
	}{
		"simple wrapping": {
			yaml:     "very-long-key-name: very-long-value-that-should-wrap",
			width:    20,
			expected: "very-long-key-name",
		},
		"no wrapping needed": {
			yaml:     "short: val",
			width:    80,
			expected: "short: val",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), true)
			renderer.SetFormatter("terminal16m")

			result, err := renderer.RenderContent(tc.yaml, tc.width)
			require.NoError(t, err)

			plainResult := ansi.Strip(result)
			assert.Contains(t, plainResult, tc.expected, "Should contain expected content")
		})
	}
}

func TestChromaRenderer_SpecialYAMLStructures(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		yaml        string
		description string
	}{
		"yaml with arrays": {
			yaml: `
items:
  - item1
  - item2
  - item3
`,
			description: "Should handle YAML arrays",
		},
		"yaml with nested objects": {
			yaml: `
outer:
  inner:
    deep:
      value: test
`,
			description: "Should handle nested YAML objects",
		},
		"yaml with multiline strings": {
			yaml: `
description: |
  This is a multiline
  string that spans
  multiple lines
`,
			description: "Should handle multiline YAML strings",
		},
		"yaml with quoted strings": {
			yaml: `
message: "This is a quoted string with special chars: !@#$%"
`,
			description: "Should handle quoted YAML strings",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			renderer := yamls.NewChromaRenderer(testTheme(), false)
			renderer.SetFormatter("terminal16m")

			result, err := renderer.RenderContent(tc.yaml, 80)
			require.NoError(t, err, tc.description)
			assert.NotEmpty(t, result, tc.description)

			// Test with search highlighting as well.
			renderer.SetSearchTerm("test")
			resultWithSearch, err := renderer.RenderContent(tc.yaml, 80)
			require.NoError(t, err, tc.description)
			assert.NotEmpty(t, resultWithSearch, tc.description)
		})
	}
}
