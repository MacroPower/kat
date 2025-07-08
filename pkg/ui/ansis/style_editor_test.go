package ansis_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/ansis"
)

func TestNewStyleEditor(t *testing.T) {
	t.Parallel()

	editor := ansis.NewStyleEditor()
	require.NotNil(t, editor)
	assert.IsType(t, &ansis.StyleEditor{}, editor)
}

func TestStyleEditor_ApplyStyles(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	editor := ansis.NewStyleEditor()
	boldStyle := lipgloss.NewStyle().Bold(true)
	italicStyle := lipgloss.NewStyle().Italic(true)
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("red"))

	tcs := map[string]struct {
		text   string
		want   string
		ranges []ansis.StyleRange
	}{
		"empty ranges": {
			text:   "hello world",
			ranges: []ansis.StyleRange{},
			want:   "hello world",
		},
		"nil ranges": {
			text:   "hello world",
			ranges: nil,
			want:   "hello world",
		},
		"single range on plain text": {
			text: "hello world",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 5, Priority: 1},
			},
			want: boldStyle.Render("hello") + " world",
		},
		"multiple non-overlapping ranges": {
			text: "hello world",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 5, Priority: 1},
				{Style: italicStyle, Start: 6, End: 11, Priority: 1},
			},
			want: boldStyle.Render("hello") + " " + italicStyle.Render("world"),
		},
		"overlapping ranges with different priorities": {
			text: "hello world",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 8, Priority: 1},
				{Style: redStyle, Start: 4, End: 11, Priority: 2},
			},
			want: boldStyle.Render("hell") + redStyle.Render("o wo") + redStyle.Render("rld"),
		},
		"range extending beyond text length": {
			text: "hello",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 100, Priority: 1},
			},
			want: boldStyle.Render("hello"),
		},
		"range starting beyond text length": {
			text: "hello",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 10, End: 20, Priority: 1},
			},
			want: "hello",
		},
		"text with existing ANSI codes": {
			text: "\x1b[31mhello\x1b[0m world",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 5, Priority: 1},
			},
			want: "\x1b[31m" + boldStyle.Render("hello") + "\x1b[31m\x1b[0m world",
		},
		"empty text": {
			text: "",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 5, Priority: 1},
			},
			want: "",
		},
		"zero-length range": {
			text: "hello world",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 5, End: 5, Priority: 1},
			},
			want: "hello world",
		},
		"complex existing ANSI with bold and color": {
			text: "\x1b[1m\x1b[31mbold red text\x1b[0m normal",
			ranges: []ansis.StyleRange{
				{Style: italicStyle, Start: 5, End: 8, Priority: 1},
			},
			want: "\x1b[1m\x1b[31mbold " + italicStyle.Render("red") + "\x1b[31m text\x1b[0m normal",
		},
		"multiple existing ANSI regions with overlapping styles": {
			text: "\x1b[1mbold\x1b[0m normal \x1b[3mitalic\x1b[0m text",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 0, End: 4, Priority: 1},    // "bold"
				{Style: boldStyle, Start: 11, End: 17, Priority: 1}, // " itali" (positions 11-16)
			},
			want: "\x1b[1m" + redStyle.Render("bold") + "\x1b[1m\x1b[0m normal" + boldStyle.Render(" itali") + "c\x1b[0m text",
		},
		"nested ANSI codes with style application": {
			text: "\x1b[1m\x1b[4mbold underline\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 5, End: 14, Priority: 1}, // "underline"
			},
			want: "\x1b[1m\x1b[4mbold " + redStyle.Render("underline") + "\x1b[4m\x1b[0m",
		},
		"text with color codes and new color overlay": {
			text: "\x1b[32mgreen text\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 0, End: 5, Priority: 1}, // "green"
			},
			want: "\x1b[32m" + redStyle.Render("green") + "\x1b[32m text\x1b[0m",
		},
		"partial overlay on existing styled text": {
			text: "\x1b[1mbold start and bold end\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: italicStyle, Start: 11, End: 14, Priority: 1}, // "and"
			},
			want: "\x1b[1mbold start " + italicStyle.Render("and") + "\x1b[1m bold end\x1b[0m",
		},
		"existing style reset in middle with new style": {
			text: "\x1b[1mbold\x1b[0m normal \x1b[1mbold again\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 5, End: 11, Priority: 1}, // "normal"
			},
			want: "\x1b[1mbold\x1b[0m " + redStyle.Render("normal") + " \x1b[1mbold again\x1b[0m",
		},
		"overlapping new styles on existing styled text": {
			text: "\x1b[31mred text here\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 8, Priority: 1},    // "red text"
				{Style: italicStyle, Start: 4, End: 13, Priority: 2}, // "text here"
			},
			want: "\x1b[31m" + boldStyle.Render("red ") + "\x1b[31m" + italicStyle.Render("text here") + "\x1b[31m\x1b[0m",
		},
		"consecutive ANSI regions with styles": {
			text: "\x1b[1mfirst\x1b[0m\x1b[3msecond\x1b[0m\x1b[4mthird\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 5, End: 11, Priority: 1}, // "second"
			},
			want: "\x1b[1mfirst\x1b[0m\x1b[3m" + redStyle.Render("second") + "\x1b[3m\x1b[0m\x1b[4mthird\x1b[0m",
		},
		"style application on text with 256 color codes": {
			text: "\x1b[38;5;196mred256\x1b[0m text",
			ranges: []ansis.StyleRange{
				{Style: boldStyle, Start: 0, End: 6, Priority: 1}, // "red256"
			},
			want: "\x1b[38;5;196m" + boldStyle.Render("red256") + "\x1b[38;5;196m\x1b[0m text",
		},
		"style application on text with RGB color codes": {
			text: "\x1b[38;2;255;0;0mrgb red\x1b[0m text",
			ranges: []ansis.StyleRange{
				{Style: italicStyle, Start: 0, End: 7, Priority: 1}, // "rgb red"
			},
			want: "\x1b[38;2;255;0;0m" + italicStyle.Render("rgb red") + "\x1b[38;2;255;0;0m\x1b[0m text",
		},
		"multiple reset sequences with style overlay": {
			text: "\x1b[1mbold\x1b[0m\x1b[0m\x1b[3mitalic\x1b[0m",
			ranges: []ansis.StyleRange{
				{Style: redStyle, Start: 4, End: 10, Priority: 1}, // space + "italic"
			},
			want: "\x1b[1mbold\x1b[0m\x1b[0m\x1b[3m" + redStyle.Render("italic") + "\x1b[3m\x1b[0m",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := editor.ApplyStyles(tc.text, tc.ranges)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStylesEqual(t *testing.T) {
	t.Parallel()

	lipgloss.SetColorProfile(termenv.TrueColor)

	boldStyle := lipgloss.NewStyle().Bold(true)
	anotherBoldStyle := lipgloss.NewStyle().Bold(true)
	italicStyle := lipgloss.NewStyle().Italic(true)
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("red"))

	tcs := map[string]struct {
		styleA lipgloss.Style
		styleB lipgloss.Style
		want   bool
	}{
		"identical styles": {
			styleA: boldStyle,
			styleB: boldStyle,
			want:   true,
		},
		"equivalent styles": {
			styleA: boldStyle,
			styleB: anotherBoldStyle,
			want:   true,
		},
		"different styles": {
			styleA: boldStyle,
			styleB: italicStyle,
			want:   false,
		},
		"different styles with different colors": {
			styleA: boldStyle,
			styleB: redStyle,
			want:   false,
		},
		"empty styles": {
			styleA: lipgloss.NewStyle(),
			styleB: lipgloss.NewStyle(),
			want:   true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := ansis.StylesEqual(tc.styleA, tc.styleB)
			assert.Equal(t, tc.want, got)
		})
	}
}
