package config

import "errors"

// TrustMode controls how project configuration trust is handled.
type TrustMode int

// TrustDecision represents the user's choice when prompted about an untrusted project.
type TrustDecision int

const (
	// TrustModePrompt prompts the user interactively (default).
	TrustModePrompt TrustMode = iota
	// TrustModeAllow trusts project configs without prompting (--trust).
	TrustModeAllow
	// TrustModeSkip skips project configs without prompting (--no-trust).
	TrustModeSkip

	// TrustSkip means the user chose to skip loading the project config.
	TrustSkip TrustDecision = iota
	// TrustAllow means the user trusts the project and wants to add it to the trust list.
	TrustAllow
)

// ErrNotInteractive is returned when a trust prompt is needed but the terminal
// is not interactive. The caller should skip loading the project config.
var ErrNotInteractive = errors.New("terminal is not interactive")

// TrustedProject represents a trusted project.
type TrustedProject struct {
	// Path is the absolute path to a trusted directory.
	Path string `json:"path" jsonschema:"title=Path"`
}

// TrustPrompter handles interactive trust prompts for project configurations.
type TrustPrompter interface {
	// Trust prompts the user to decide whether to trust a project configuration.
	// Returns [TrustDecision] and any error (including [ErrNotInteractive]).
	Trust(projectDir, configPath string) (TrustDecision, error)
}
