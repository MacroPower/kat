package kube

type Config struct {
	Commands []*Command
}

var DefaultConfig = Config{
	Commands: []*Command{
		MustNewCommand(".*/Chart\\.ya?ml", "helm", "template", ".", "--generate-name", "--debug"),
		MustNewCommand(".*/kustomization\\.ya?ml", "kustomize", "build", "."),
	},
}
