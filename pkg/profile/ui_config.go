package profile

// UIConfig defines UI config overrides for a profile.
type UIConfig struct {
	Compact     *bool  `json:"compact,omitempty"`
	WordWrap    *bool  `json:"wordWrap,omitempty"`
	LineNumbers *bool  `json:"lineNumbers,omitempty"`
	Theme       string `json:"theme,omitempty"`
}
