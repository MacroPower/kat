package kube

import (
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/MacroPower/kat/pkg/profile"
	"github.com/MacroPower/kat/pkg/rule"
)

var (
	defaultProfiles = map[string]*profile.Profile{
		"ks": profile.MustNew("kustomize",
			profile.WithArgs("build", "."),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			profile.WithHooks(
				profile.NewHooks(
					profile.WithInit(
						profile.NewHookCommand("kustomize", "version"),
					),
				),
			)),
		"helm": profile.MustNew("helm",
			profile.WithArgs("template", ".", "--generate-name"),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])`),
			profile.WithHooks(
				profile.NewHooks(
					profile.WithInit(
						profile.NewHookCommand("helm", "version", "--short"),
					),
					profile.WithPreRender(
						profile.NewHookCommand("helm", "dependency", "build"),
					),
				),
			)),
		"yaml": profile.MustNew("sh",
			profile.WithArgs("-c", "yq eval-all '.' *.yaml"),
			profile.WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			profile.WithHooks(
				profile.NewHooks(
					profile.WithInit(
						profile.NewHookCommand("yq", "-V"),
					),
				),
			)),
	}

	defaultRules = []*rule.Rule{
		rule.MustNew("ks", `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`),
		rule.MustNew("helm", `files.exists(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])`),
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	}

	DefaultConfig = MustNewConfig(defaultProfiles, defaultRules)
)

type Config struct {
	Profiles map[string]*profile.Profile `validate:"dive" yaml:"profiles,omitempty"`
	Rules    []*rule.Rule                `validate:"dive" yaml:"rules,omitempty"`
}

type ConfigError struct {
	Path *yaml.Path // YAML path to the error.
	Err  error
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("error at %s: %v", e.Path.String(), e.Err)
}

func NewConfig(ps map[string]*profile.Profile, rs []*rule.Rule) (*Config, error) {
	c := &Config{
		Profiles: ps,
		Rules:    rs,
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return c, nil
}

func MustNewConfig(ps map[string]*profile.Profile, rs []*rule.Rule) *Config {
	c, err := NewConfig(ps, rs)
	if err != nil {
		panic(fmt.Sprintf("failed to create config: %v", err))
	}

	return c
}

func (c *Config) EnsureDefaults() {
	if c.Profiles == nil {
		c.Profiles = defaultProfiles
	}
	if c.Rules == nil {
		c.Rules = defaultRules
	}
}

func (c *Config) Validate() *ConfigError {
	pb := yaml.PathBuilder{}

	for name, p := range c.Profiles {
		if err := p.CompileSource(); err != nil {
			return &ConfigError{
				Path: pb.Root().Child("profiles").Child(name).Child("source").Build(),
				Err:  fmt.Errorf("invalid source: %w", err),
			}
		}
	}

	for i, r := range c.Rules {
		uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
		if err := r.CompileMatch(); err != nil {
			return &ConfigError{
				Path: pb.Root().Child("rules").Index(uIdx).Child("match").Build(),
				Err:  fmt.Errorf("invalid match: %w", err),
			}
		}
		p, ok := c.Profiles[r.Profile]
		if !ok {
			return &ConfigError{
				Path: pb.Root().Child("rules").Index(uIdx).Child("profile").Build(),
				Err:  fmt.Errorf("profile %q not found", r.Profile),
			}
		}
		r.SetProfile(p)
	}

	return nil
}
