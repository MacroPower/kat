package profile

// UIConfig defines UI config overrides for a profile.
type UIConfig struct {
	Compact     *bool  `yaml:"compact"`
	WordWrap    *bool  `yaml:"wordWrap"`
	LineNumbers *bool  `yaml:"lineNumbers"`
	Theme       string `yaml:"theme"`
}
