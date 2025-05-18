package config

import (
	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui"
)

type Config struct {
	UI       ui.Config
	Commands kube.Config
}
