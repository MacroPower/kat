package kube

type Config struct {
	Commands []*Command `json:"commands" kong:"-" yaml:"commands"`
}

var DefaultConfig = Config{
	Commands: []*Command{
		MustNewCommand(nil, ".*/Chart\\.ya?ml", "helm", "template", ".", "--generate-name"),
		MustNewCommand(nil, ".*/kustomization\\.ya?ml", "kustomize", "build", "."),
	},
}
