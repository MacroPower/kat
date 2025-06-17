package kube

type Config struct {
	Commands []*Command `json:"commands" kong:"-" yaml:"commands"`
}

func (c *Config) EnsureDefaults() {
	if c.Commands == nil {
		c.Commands = DefaultConfig.Commands
	}
}

var DefaultConfig = Config{
	Commands: []*Command{
		MustNewCommand(
			NewHooks(
				WithInit(
					NewHookCommand("helm", "version", "--short"),
				),
				WithPreRender(
					NewHookCommand("helm", "dependency", "build"),
				),
			),
			".*/Chart\\.ya?ml$", ".*\\.(ya?ml|tpl)$",
			"helm", "template", ".", "--generate-name",
		),
		MustNewCommand(
			NewHooks(
				WithInit(
					NewHookCommand("kustomize", "version"),
				),
			),
			".*/kustomization\\.ya?ml$", ".*\\.ya?ml$",
			"kustomize", "build", ".",
		),
	},
}
