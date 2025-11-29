// Package policy provides trust management for project configurations.
package policy

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/macropower/kat/api/v1beta1/policies"
	"github.com/macropower/kat/api/v1beta1/runtimeconfigs"
	"github.com/macropower/kat/pkg/config"
)

// TrustMode controls how runtime configuration trust is handled.
type TrustMode int

// TrustDecision represents the user's choice when prompted about an untrusted project.
type TrustDecision int

const (
	// TrustModePrompt prompts the user interactively (default).
	TrustModePrompt TrustMode = iota
	// TrustModeAllow trusts runtime configs without prompting (--trust).
	TrustModeAllow
	// TrustModeSkip skips runtime configs without prompting (--no-trust).
	TrustModeSkip
)

const ( //nolint:grouper // Separate iota sequences require separate const blocks.
	// TrustDecisionSkip means the user chose to skip loading the runtime config.
	TrustDecisionSkip TrustDecision = iota
	// TrustDecisionAllow means the user trusts the project and wants to add it to the trust list.
	TrustDecisionAllow
)

// ErrNotInteractive is returned when a trust prompt is needed but the terminal
// is not interactive. The caller should skip loading the runtime config.
var ErrNotInteractive = errors.New("terminal is not interactive")

// TrustPrompter handles interactive trust prompts for project configurations.
type TrustPrompter interface {
	// Prompt prompts the user to decide whether to trust a project configuration.
	// Returns [TrustDecision] and any error (including [ErrNotInteractive]).
	Prompt(projectDir, configPath string) (TrustDecision, error)
}

// TrustManager handles trust decisions for project configurations.
type TrustManager struct {
	policy     *policies.Policy
	policyPath string
}

// NewTrustManager creates a new [TrustManager].
func NewTrustManager(pol *policies.Policy, policyPath string) *TrustManager {
	if pol == nil {
		pol = policies.New()
	}

	return &TrustManager{
		policy:     pol,
		policyPath: policyPath,
	}
}

// LoadTrustedRuntimeConfig finds and loads a runtime config if it exists and is trusted.
// Returns nil (not an error) if no runtime config found or if untrusted.
//
//nolint:nilnil // Returning nil with nil error is intentional for "not found" and "untrusted" cases.
func (m *TrustManager) LoadTrustedRuntimeConfig(
	targetPath string,
	prompter TrustPrompter,
	mode TrustMode,
) (*runtimeconfigs.RuntimeConfig, error) {
	runtimeCfgPath, err := runtimeconfigs.Find(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("find runtime config: %w", err)
	}

	if runtimeCfgPath == "" {
		return nil, nil
	}

	projectDir := filepath.Dir(runtimeCfgPath)

	trusted, err := m.ensureTrusted(projectDir, runtimeCfgPath, prompter, mode)
	if err != nil {
		return nil, err
	}

	if !trusted {
		slog.Warn("skipping untrusted runtime configuration", slog.String("path", runtimeCfgPath))

		return nil, nil
	}

	loader, err := config.NewLoaderFromFile(
		runtimeCfgPath,
		runtimeconfigs.New,
		runtimeconfigs.DefaultValidator,
		config.WithThemeFromData(),
	)
	if err != nil {
		return nil, fmt.Errorf("create runtime loader: %w", err)
	}

	err = loader.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate runtime config %q: %w", runtimeCfgPath, err)
	}

	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("load runtime config %q: %w", runtimeCfgPath, err)
	}

	// Validate business logic after loading.
	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate runtime config %q: %w", runtimeCfgPath, err)
	}

	slog.Debug("loaded runtime configuration", slog.String("path", runtimeCfgPath))

	return cfg, nil
}

func (m *TrustManager) ensureTrusted(
	projectDir, runtimeCfgPath string,
	prompter TrustPrompter,
	mode TrustMode,
) (bool, error) {
	switch mode {
	case TrustModeSkip:
		slog.Info("skipping runtime config (--no-trust)", slog.String("path", runtimeCfgPath))

		return false, nil

	case TrustModeAllow:
		slog.Info("trusting runtime config (--trust)", slog.String("path", runtimeCfgPath))

		err := m.policy.TrustProject(projectDir, m.policyPath)
		if err != nil {
			slog.Warn("could not save trusted project", slog.Any("err", err))
		}

		return true, nil

	case TrustModePrompt:
		// Check if already trusted in policy.
		if m.policy.IsTrusted(projectDir) {
			return true, nil
		}

		if prompter == nil {
			slog.Warn(
				"skipping untrusted runtime config (no prompter)",
				slog.String("path", runtimeCfgPath),
			)

			return false, nil
		}

		decision, err := prompter.Prompt(projectDir, runtimeCfgPath)
		if errors.Is(err, ErrNotInteractive) {
			slog.Warn(
				"skipping untrusted runtime config (non-interactive)",
				slog.String("path", runtimeCfgPath),
				slog.String(
					"hint",
					"run kat interactively to trust this project, or use --trust/--no-trust flags",
				),
			)

			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("prompt: %w", err)
		}

		if decision == TrustDecisionSkip {
			return false, nil
		}

		err = m.policy.TrustProject(projectDir, m.policyPath)
		if err != nil {
			slog.Warn("could not save trusted project", slog.Any("err", err))
		}

		return true, nil

	default:
		return false, fmt.Errorf("unknown trust mode: %d", mode)
	}
}
