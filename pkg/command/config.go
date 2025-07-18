package command

import (
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/macropower/kat/pkg/execs"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

const (
	filterYAMLFiles = `files.filter(f, pathExt(f) in [".yaml", ".yml"])`
	filterHelmFiles = `files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])`

	existsKustomizeProject = `files.exists(f,
  pathBase(f) in ["kustomization.yaml", "kustomization.yml"])`

	existsHelmV3Project = `files.exists(f,
  pathBase(f) in ["Chart.yaml", "Chart.yml"] &&
  yamlPath(f, "$.apiVersion") == "v2")`

	existsYAMLFiles = `files.exists(f,
  pathExt(f) in [".yaml", ".yml"])`
)

var (
	defaultProfiles = map[string]*profile.Profile{
		"ks": profile.MustNew("kustomize",
			profile.WithArgs("build", "."),
			profile.WithSource(filterYAMLFiles),
			profile.WithHooks(
				profile.MustNewHooks(
					profile.WithInit(
						profile.MustNewHookCommand("kustomize", profile.WithHookArgs("version")),
					),
				),
			)),
		"ks-helm": profile.MustNew("kustomize",
			profile.WithArgs("build", ".", "--enable-helm"),
			profile.WithSource(filterYAMLFiles),
			profile.WithHooks(
				profile.MustNewHooks(
					profile.WithInit(
						profile.MustNewHookCommand("kustomize", profile.WithHookArgs("version")),
					),
				),
			)),
		"helm": profile.MustNew("helm",
			profile.WithArgs("template", "."),
			profile.WithExtraArgs("-g"),
			profile.WithSource(filterHelmFiles),
			profile.WithEnvFrom([]execs.EnvFromSource{
				{
					CallerRef: &execs.CallerRef{
						Pattern: "^HELM_.+",
					},
				},
			}),
			profile.WithHooks(
				profile.MustNewHooks(
					profile.WithInit(
						profile.MustNewHookCommand("helm", profile.WithHookArgs("version", "--short")),
					),
					profile.WithPreRender(
						profile.MustNewHookCommand("helm",
							profile.WithHookArgs("dependency", "build"),
							profile.WithHookEnvFrom([]execs.EnvFromSource{
								{
									CallerRef: &execs.CallerRef{
										Pattern: "^HELM_.+",
									},
								},
							}),
						),
					),
				),
			)),
		"yaml": profile.MustNew("sh",
			profile.WithArgs("-c", "yq eval-all '.' *.yaml"),
			profile.WithSource(filterYAMLFiles),
			profile.WithHooks(
				profile.MustNewHooks(
					profile.WithInit(
						profile.MustNewHookCommand("yq", profile.WithHookArgs("-V")),
					),
				),
			)),
	}

	defaultRules = []*rule.Rule{
		rule.MustNew("ks", existsKustomizeProject),
		rule.MustNew("helm", existsHelmV3Project),
		rule.MustNew("yaml", existsYAMLFiles),
	}

	DefaultConfig = MustNewConfig(defaultProfiles, defaultRules)
)

// Config defines the core (non-UI) kat configuration.
type Config struct {
	// Profiles contains a map of profile names to profile configurations.
	Profiles map[string]*profile.Profile `json:"profiles,omitempty" jsonschema:"title=Profiles"`
	// Rules defines the rules for matching files to profiles.
	Rules []*rule.Rule `json:"rules,omitempty" jsonschema:"title=Rules"`
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
	err := c.Validate()
	if err != nil {
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
		err := p.CompileSource()
		if err != nil {
			return &ConfigError{
				Path: pb.Root().Child("profiles").Child(name).Child("source").Build(),
				Err:  fmt.Errorf("invalid source: %w", err),
			}
		}

		for i, env := range p.Command.Env {
			if env.ValueFrom == nil || env.ValueFrom.CallerRef == nil || env.ValueFrom.CallerRef.Pattern == "" {
				continue // Skip if no pattern is defined.
			}

			uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
			err := env.ValueFrom.CallerRef.Compile()
			if err != nil {
				return &ConfigError{
					Path: pb.Root().
						Child("profiles").
						Child(name).
						Child("env").
						Index(uIdx).
						Child("valueFrom").
						Child("callerRef").
						Child("pattern").
						Build(),
					Err: fmt.Errorf("invalid env pattern: %w", err),
				}
			}
		}

		for i, envFrom := range p.Command.EnvFrom {
			if envFrom.CallerRef == nil || envFrom.CallerRef.Pattern == "" {
				continue // Skip if no pattern is defined.
			}

			uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
			err := envFrom.CallerRef.Compile()
			if err != nil {
				return &ConfigError{
					Path: pb.Root().
						Child("profiles").
						Child(name).
						Child("envFrom").
						Index(uIdx).
						Child("callerRef").
						Child("pattern").
						Build(),
					Err: fmt.Errorf("invalid envFrom pattern: %w", err),
				}
			}
		}
		// TODO: Build should return *ConfigError to avoid the duplicate validation above.
		err = p.Build()
		if err != nil {
			return &ConfigError{
				Path: pb.Root().Child("profiles").Child(name).Build(),
				Err:  fmt.Errorf("invalid profile: %w", err),
			}
		}
	}

	for i, r := range c.Rules {
		uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
		err := r.CompileMatch()
		if err != nil {
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
