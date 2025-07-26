package yamls_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/ui/yamls"
)

func TestNewErrorHighlighter(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	highlighter := yamls.NewErrorHighlighter(errorStyle)

	assert.NotNil(t, highlighter)
}

func TestErrorHighlighter_ApplyErrorHighlights(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	errorStyle := lipgloss.NewStyle().Background(lipgloss.Color("1"))
	highlighter := yamls.NewErrorHighlighter(errorStyle)

	tcs := map[string]struct {
		input  string
		want   string
		errors []yamls.ErrorPosition
	}{
		"empty text and no errors": {
			input:  "",
			errors: []yamls.ErrorPosition{},
			want:   "",
		},
		"text with no errors": {
			input:  "hello world",
			errors: []yamls.ErrorPosition{},
			want:   "hello world",
		},
		"single line with error highlight": {
			input: "hello world",
			errors: []yamls.ErrorPosition{
				{Line: 0, Start: 6, End: 11},
			},
			want: "hello \x1b[41mworld\x1b[0m",
		},
		"multiple lines with error highlights": {
			input: "line one\nline two\nline three",
			errors: []yamls.ErrorPosition{
				{Line: 0, Start: 0, End: 4},
				{Line: 2, Start: 5, End: 10},
			},
			want: "\x1b[41mline\x1b[0m one\nline two\nline \x1b[41mthree\x1b[0m",
		},
		"overlapping error highlights on same line": {
			input: "hello world test",
			errors: []yamls.ErrorPosition{
				{Line: 0, Start: 6, End: 11},
				{Line: 0, Start: 9, End: 16},
			},
			want: "hello \x1b[41mworld test\x1b[0m",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := highlighter.ApplyErrorHighlights(tc.input, tc.errors)
			assert.Equal(t, tc.want, got)
		})
	}
}
