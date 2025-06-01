package kube

type Config struct {
	Commands []*Command `json:"commands" kong:"-" yaml:"commands"`
}

var DefaultConfig = Config{
	Commands: []*Command{
		MustNewCommand(".*/Chart\\.ya?ml", "helm", "template", ".", "--generate-name"),
		MustNewCommand(".*/kustomization\\.ya?ml", "kustomize", "build", "."),
	},
}
