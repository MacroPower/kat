package ui

// Config contains TUI-specific configuration.
type Config struct {
	Gopath          string `env:"GOPATH"`
	HomeDir         string `env:"HOME"`
	GlamourStyle    string `env:"GLAMOUR_STYLE"`
	Path            string // Working directory or file path.
	GlamourMaxWidth uint
	ShowAllFiles    bool
	ShowLineNumbers bool
	EnableMouse     bool
	GlamourEnabled  bool `env:"GLAMOUR_ENABLED" envDefault:"true"` // For debugging the UI.
}
