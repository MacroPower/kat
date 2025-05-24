package config

// Config contains TUI-specific configuration.
type Config struct {
	GlamourStyle    string `yaml:"glamour-style"`
	GlamourMaxWidth int    `yaml:"glamour-max-width"`
	GlamourDisabled bool   `yaml:"glamour-disabled"` // For debugging the UI.
	ShowLineNumbers bool   `yaml:"show-line-numbers"`
	EnableMouse     bool   `yaml:"enable-mouse"`
}

var DefaultConfig = Config{
	GlamourStyle:    "auto",
	ShowLineNumbers: true,
	EnableMouse:     true,
}
