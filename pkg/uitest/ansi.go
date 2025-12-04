package uitest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// SetupColorProfile sets the color profile to TrueColor for consistent test output.
// Call this at the start of tests that involve styled output.
func SetupColorProfile() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// StyleExpectation defines expected ANSI style attributes.
type StyleExpectation struct {
	Bold       *bool   // Whether text should be bold.
	Italic     *bool   // Whether text should be italic.
	Underline  *bool   // Whether text should be underlined.
	Foreground *string // Expected foreground color code (e.g., "212", "99").
	Background *string // Expected background color code.
}

// ANSIStyleVerifier helps verify ANSI escape sequences in output.
type ANSIStyleVerifier struct {
	output string
}

// NewANSIStyleVerifier creates a new verifier for the given output.
func NewANSIStyleVerifier(output string) *ANSIStyleVerifier {
	return &ANSIStyleVerifier{output: output}
}

// PlainText strips all ANSI sequences and returns plain text.
func (v *ANSIStyleVerifier) PlainText() string {
	return ansi.Strip(v.output)
}

// ContainsPlainText checks if the plain text (ANSI stripped) contains the expected string.
func (v *ANSIStyleVerifier) ContainsPlainText(t *testing.T, expected string) {
	t.Helper()

	assert.Contains(t, v.PlainText(), expected, "plain text should contain %q", expected)
}

// ContainsStyledText verifies that text appears with expected styling.
// It searches for the text and checks if it's preceded by ANSI sequences
// matching the expected style.
func (v *ANSIStyleVerifier) ContainsStyledText(t *testing.T, text string, expected StyleExpectation) {
	t.Helper()

	// First verify the text exists.
	assert.Contains(t, v.PlainText(), text, "output should contain %q", text)

	// Parse the output and find styled segments.
	segments := v.parseStyledSegments()

	// Find segments containing the target text.
	found := false
	for _, seg := range segments {
		if strings.Contains(seg.text, text) {
			found = true

			v.verifyStyleExpectation(t, text, seg.style, expected)

			break
		}
	}

	if !found {
		t.Errorf("could not find styled segment containing %q", text)
	}
}

// styledSegment represents a piece of text with its associated ANSI style.
type styledSegment struct {
	text  string
	style parsedStyle
}

// parsedStyle holds parsed ANSI style attributes.
type parsedStyle struct {
	foreground string
	background string
	sgrParams  []int
	bold       bool
	italic     bool
	underline  bool
}

// parseStyledSegments parses the output into segments with their styles.
func (v *ANSIStyleVerifier) parseStyledSegments() []styledSegment {
	var (
		segments     []styledSegment
		currentStyle parsedStyle
		currentText  strings.Builder
	)

	input := []byte(v.output)

	var state byte

	p := ansi.GetParser()
	defer ansi.PutParser(p)

	for len(input) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(input, state, p)

		if ansi.HasCsiPrefix(seq) && len(seq) > 0 && seq[len(seq)-1] == 'm' {
			// SGR sequence - save current segment and update style.
			if currentText.Len() > 0 {
				segments = append(segments, styledSegment{
					text:  currentText.String(),
					style: currentStyle,
				})
				currentText.Reset()
			}

			currentStyle = v.parseSGR(p, currentStyle)
		} else if width > 0 {
			// Regular character.
			currentText.Write(seq)
		}

		input = input[n:]
		state = newState
	}

	// Don't forget the last segment.
	if currentText.Len() > 0 {
		segments = append(segments, styledSegment{
			text:  currentText.String(),
			style: currentStyle,
		})
	}

	return segments
}

// parseSGR parses SGR parameters from the parser state.
func (v *ANSIStyleVerifier) parseSGR(p *ansi.Parser, current parsedStyle) parsedStyle {
	style := current
	params := p.Params()

	for i := 0; i < len(params); i++ {
		param := params[i].Param(0)

		switch param {
		case 0: // Reset.
			style = parsedStyle{}
		case 1: // Bold.
			style.bold = true
		case 3: // Italic.
			style.italic = true
		case 4: // Underline.
			style.underline = true
		case 22: // Normal intensity (not bold).
			style.bold = false
		case 23: // Not italic.
			style.italic = false
		case 24: // Not underlined.
			style.underline = false
		case 38: // Foreground color.
			if i+1 < len(params) {
				colorType := params[i+1].Param(0)
				switch colorType {
				case 5: // 256 color.
					if i+2 < len(params) {
						style.foreground = formatColorCode(params[i+2].Param(0))
						i += 2
					}

				case 2: // RGB.
					if i+4 < len(params) {
						r := params[i+2].Param(0)
						g := params[i+3].Param(0)
						b := params[i+4].Param(0)
						style.foreground = formatRGB(r, g, b)
						i += 4
					}
				}
			}

		case 48: // Background color.
			if i+1 < len(params) {
				colorType := params[i+1].Param(0)
				switch colorType {
				case 5: // 256 color.
					if i+2 < len(params) {
						style.background = formatColorCode(params[i+2].Param(0))
						i += 2
					}

				case 2: // RGB.
					if i+4 < len(params) {
						r := params[i+2].Param(0)
						g := params[i+3].Param(0)
						b := params[i+4].Param(0)
						style.background = formatRGB(r, g, b)
						i += 4
					}
				}
			}

		default:
			switch { // Handle basic ANSI colors.
			case param >= 30 && param <= 37: // Normal foreground colors.
				style.foreground = formatColorCode(param - 30)
			case param >= 40 && param <= 47: // Normal background colors.
				style.background = formatColorCode(param - 40)
			case param >= 90 && param <= 97: // Bright foreground colors.
				style.foreground = formatColorCode(param - 90 + 8)
			case param >= 100 && param <= 107: // Bright background colors.
				style.background = formatColorCode(param - 100 + 8)
			}
		}

		style.sgrParams = append(style.sgrParams, param)
	}

	return style
}

// formatColorCode formats a color code as a string.
func formatColorCode(code int) string {
	return strconv.Itoa(code)
}

// formatRGB formats RGB values as a hex string.
func formatRGB(r, g, b int) string {
	return strings.ToUpper(strings.TrimPrefix(
		"#"+hexByte(r)+hexByte(g)+hexByte(b),
		"#",
	))
}

func hexByte(v int) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{hex[v/16], hex[v%16]})
}

// verifyStyleExpectation checks if the parsed style matches expectations.
func (v *ANSIStyleVerifier) verifyStyleExpectation(
	t *testing.T,
	text string,
	style parsedStyle,
	expected StyleExpectation,
) {
	t.Helper()

	if expected.Bold != nil {
		assert.Equal(t, *expected.Bold, style.bold,
			"text %q: expected bold=%v, got bold=%v", text, *expected.Bold, style.bold)
	}

	if expected.Italic != nil {
		assert.Equal(t, *expected.Italic, style.italic,
			"text %q: expected italic=%v, got italic=%v", text, *expected.Italic, style.italic)
	}

	if expected.Underline != nil {
		assert.Equal(t, *expected.Underline, style.underline,
			"text %q: expected underline=%v, got underline=%v", text, *expected.Underline, style.underline)
	}

	if expected.Foreground != nil {
		assert.Equal(t, *expected.Foreground, style.foreground,
			"text %q: expected foreground=%q, got foreground=%q", text, *expected.Foreground, style.foreground)
	}

	if expected.Background != nil {
		assert.Equal(t, *expected.Background, style.background,
			"text %q: expected background=%q, got background=%q", text, *expected.Background, style.background)
	}
}

// Ptr is a helper to create a pointer to a value.
// Useful for setting optional fields in StyleExpectation.
func Ptr[T any](v T) *T {
	return &v
}
