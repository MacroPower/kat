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
				WithPreRender(
					NewHookCommand("helm", "dependency", "build"),
				),
			),
			".*/Chart\\.ya?ml$", ".*\\.(ya?ml|tpl)$",
			"helm", "template", ".", "--generate-name",
		),
		MustNewCommand(
			nil,
			".*/kustomization\\.ya?ml$", ".*\\.ya?ml$",
			"kustomize", "build", ".",
		),
	},
}
