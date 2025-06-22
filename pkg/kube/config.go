package kube

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

var (
	defaultProfiles = map[string]*Profile{
		"ks": MustNewProfile("kustomize",
			WithArgs("build", "."),
			WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			WithHooks(
				NewHooks(
					WithInit(
						NewHookCommand("kustomize", "version"),
					),
				),
			)),
		"helm": MustNewProfile("helm",
			WithArgs("template", ".", "--generate-name"),
			WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])`),
			WithHooks(
				NewHooks(
					WithInit(
						NewHookCommand("helm", "version", "--short"),
					),
					WithPreRender(
						NewHookCommand("helm", "dependency", "build"),
					),
				),
			)),
		"yaml": MustNewProfile("sh",
			WithArgs("-c", "yq eval-all '.' *.yaml"),
			WithSource(`files.filter(f, pathExt(f) in [".yaml", ".yml"])`),
			WithHooks(
				NewHooks(
					WithInit(
						NewHookCommand("yq", "-V"),
					),
				),
			)),
	}

	defaultRules = []*Rule{
		MustNewRule("ks", `files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`),
		MustNewRule("helm", `files.exists(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])`),
		MustNewRule("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	}

	DefaultConfig = MustNewConfig(defaultProfiles, defaultRules)
)

type Config struct {
	Profiles map[string]*Profile `validate:"dive" yaml:"profiles,omitempty"`
	Rules    []*Rule             `validate:"dive" yaml:"rules,omitempty"`
}

type ConfigError struct {
	Path *yaml.Path // YAML path to the error.
	Err  error
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("error at %s: %v", e.Path.String(), e.Err)
}

func NewConfig(profiles map[string]*Profile, rules []*Rule) (*Config, error) {
	c := &Config{
		Profiles: profiles,
		Rules:    rules,
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return c, nil
}

func MustNewConfig(profiles map[string]*Profile, rules []*Rule) *Config {
	c, err := NewConfig(profiles, rules)
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

	for name, profile := range c.Profiles {
		if err := profile.CompileSource(); err != nil {
			return &ConfigError{
				Path: pb.Root().Child("profiles").Child(name).Child("source").Build(),
				Err:  fmt.Errorf("invalid source: %w", err),
			}
		}
	}

	for i, rule := range c.Rules {
		uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
		if err := rule.CompileMatch(); err != nil {
			return &ConfigError{
				Path: pb.Root().Child("rules").Index(uIdx).Child("match").Build(),
				Err:  fmt.Errorf("invalid match: %w", err),
			}
		}
		profile, ok := c.Profiles[rule.Profile]
		if !ok {
			return &ConfigError{
				Path: pb.Root().Child("rules").Index(uIdx).Child("profile").Build(),
				Err:  fmt.Errorf("profile %q not found", rule.Profile),
			}
		}
		rule.SetProfile(profile)
	}

	return nil
}
