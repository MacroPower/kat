package profile

// UIConfig defines UI config overrides for a profile.
type UIConfig struct {
	// Compact enables compact display mode with reduced spacing.
	Compact *bool `json:"compact,omitempty" jsonschema:"title=Enable Compact Display"`
	// WordWrap enables automatic word wrapping for long text.
	WordWrap *bool `json:"wordWrap,omitempty" jsonschema:"title=Enable Word Wrap"`
	// LineNumbers enables line numbers in the display.
	LineNumbers *bool `json:"lineNumbers,omitempty" jsonschema:"title=Enable Line Numbers"`
	// Theme specifies the theme name to use. This can be a custom theme added under `themes`,
	// or a theme from the Chroma Style Gallery: https://xyproto.github.io/splash/docs/
	Theme string `json:"theme,omitempty" jsonschema:"title=Theme Name"`
}
