package command

import (
	"fmt"
	"maps"

	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/paths"

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

// Config defines the core (non-UI) kat configuration.
type Config struct {
	// Profiles contains a map of profile names to profile configurations.
	Profiles map[string]*profile.Profile `json:"profiles,omitempty" jsonschema:"title=Profiles"`
	// Rules defines the rules for matching files to profiles.
	Rules []*rule.Rule `json:"rules,omitempty" jsonschema:"title=Rules"`
}

// NewConfig creates a new [Config] with default profiles and rules.
func NewConfig() *Config {
	c := &Config{}
	c.EnsureDefaults()

	return c
}

func (c *Config) EnsureDefaults() {
	if c.Profiles == nil {
		c.Profiles = map[string]*profile.Profile{
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
	}
	if c.Rules == nil {
		c.Rules = []*rule.Rule{
			rule.MustNew("ks", existsKustomizeProject),
			rule.MustNew("helm", existsHelmV3Project),
			rule.MustNew("yaml", existsYAMLFiles),
		}
	}
}

// Merge merges another Config into this one.
// Project profiles override global profiles with the same key.
// Project rules are prepended to global rules (evaluated first, higher priority).
func (c *Config) Merge(project *Config) {
	if project == nil {
		return
	}

	// Initialize maps/slices if needed.
	if c.Profiles == nil {
		c.Profiles = make(map[string]*profile.Profile)
	}

	// Project profiles override global profiles with the same key.
	maps.Copy(c.Profiles, project.Profiles)

	// Project rules are prepended (evaluated first, higher priority).
	if len(project.Rules) > 0 {
		c.Rules = append(project.Rules, c.Rules...)
	}
}

func (c *Config) Validate() error {
	for name, p := range c.Profiles {
		err := p.CompileSource()
		if err != nil {
			return niceyaml.NewErrorFrom(
				fmt.Errorf("invalid source for profile %q: %w", name, err),
				niceyaml.WithPath(paths.Root().Child("profiles", name, "source").Key()),
			)
		}

		for i, env := range p.Command.Env {
			if env.ValueFrom == nil || env.ValueFrom.CallerRef == nil || env.ValueFrom.CallerRef.Pattern == "" {
				continue // Skip if no pattern is defined.
			}

			uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
			err := env.ValueFrom.CallerRef.Compile()
			if err != nil {
				return niceyaml.NewErrorFrom(
					fmt.Errorf("invalid env pattern: %w", err),
					niceyaml.WithPath(paths.Root().
						Child("profiles", name, "env").
						Index(uIdx).
						Child("valueFrom", "callerRef", "pattern").
						Key()),
				)
			}
		}

		for i, envFrom := range p.Command.EnvFrom {
			if envFrom.CallerRef == nil || envFrom.CallerRef.Pattern == "" {
				continue // Skip if no pattern is defined.
			}

			uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
			err := envFrom.CallerRef.Compile()
			if err != nil {
				return niceyaml.NewErrorFrom(
					fmt.Errorf("invalid envFrom pattern: %w", err),
					niceyaml.WithPath(paths.Root().
						Child("profiles", name, "envFrom").
						Index(uIdx).
						Child("callerRef", "pattern").
						Key()),
				)
			}
		}
		// TODO: Build should return *ConfigError to avoid the duplicate validation above.
		err = p.Build()
		if err != nil {
			return niceyaml.NewErrorFrom(
				fmt.Errorf("invalid profile: %w", err),
				niceyaml.WithPath(paths.Root().Child("profiles", name).Key()),
			)
		}
	}

	for i, r := range c.Rules {
		uIdx := uint(i) //nolint:gosec // G115: integer overflow conversion int -> uint.
		err := r.CompileMatch()
		if err != nil {
			return niceyaml.NewErrorFrom(
				fmt.Errorf("invalid match: %w", err),
				niceyaml.WithPath(paths.Root().Child("rules").Index(uIdx).Child("match").Key()),
			)
		}

		_, ok := c.Profiles[r.Profile]
		if !ok {
			return niceyaml.NewErrorFrom(
				fmt.Errorf("profile %q not found", r.Profile),
				niceyaml.WithPath(paths.Root().Child("rules").Index(uIdx).Child("profile").Key()),
			)
		}
	}

	return nil
}
