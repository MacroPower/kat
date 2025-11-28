package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/ui/theme"
)

// Prompter handles interactive prompts during initialization.
type Prompter struct {
	t *theme.Theme
}

// NewPrompter creates a new setup prompter.
func NewPrompter(t *theme.Theme) *Prompter {
	return &Prompter{
		t: t,
	}
}

// Trust displays a CLI prompt asking the user about project trust.
func (p *Prompter) Trust(projectPath, configPath string) (config.TrustDecision, error) {
	ctx := context.Background()

	// Check if we're running interactively.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return config.TrustSkip, config.ErrNotInteractive
	}

	var decision string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Project Configuration Found").
				Description(fmt.Sprintf(
					"A project configuration was found at:\n%s\n\n"+
						"Project directory:\n%s\n\n"+
						"This project wants to define custom rendering rules and profiles.\n"+
						"Do you trust this project?",
					configPath,
					projectPath,
				)),

			huh.NewSelect[string]().
				Options(
					huh.NewOption("Trust (add to trusted projects)", "trust"),
					huh.NewOption("Skip (use global config only)", "skip"),
				).
				Value(&decision),
		),
	).
		WithShowHelp(false).
		WithTheme(theme.HuhTheme(p.t))

	err := form.RunWithContext(ctx)
	if err != nil {
		return config.TrustSkip, fmt.Errorf("run trust prompt: %w", err)
	}

	if decision == "trust" {
		return config.TrustAllow, nil
	}

	return config.TrustSkip, nil
}
