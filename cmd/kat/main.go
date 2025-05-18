package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/MacroPower/kat/pkg/config"
	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/log"
	"github.com/MacroPower/kat/pkg/ui"
)

const (
	cmdName = "kat"
	cmdDesc = `cat for Kubernetes resources.`

	configPath = "$XDG_CONFIG_HOME/kat/config.yaml"
)

var cli struct {
	Path string `arg:"" optional:"" type:"path" help:"File or directory path."`

	Config config.Config `embed:""`

	Log struct {
		Level  string `default:"info"   help:"Log level."`
		Format string `default:"logfmt" help:"Log format. One of: [logfmt, json]"`
	} `embed:"" prefix:"log-"`
}

func main() {
	configPathExp := os.ExpandEnv(configPath)
	err := config.NewConfig().Write(configPathExp)

	cliCtx := kong.Parse(&cli,
		kong.Name(cmdName),
		kong.Description(cmdDesc),
		kong.DefaultEnvars("KAT"),
		kong.Configuration(kongyaml.Loader, configPathExp),
	)

	if err != nil {
		cliCtx.Fatalf("failed to initialize config: %v", err)
	}

	logHandler, err := log.CreateHandlerWithStrings(cliCtx.Stderr, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}
	slog.SetDefault(slog.New(logHandler))

	path, err := filepath.Abs(cli.Path)
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}

	cmd := kube.NewCommandRunner(path, kube.WithCommands(cli.Config.Kube.Commands))

	// Hack: make sure that we can run the command.
	// TODO: implement proper error handling in the UI.
	if _, err = cmd.Run(); err != nil {
		cliCtx.FatalIfErrorf(err)
	}

	p := ui.NewProgram(cli.Config.UI, cmd)
	if _, err := p.Run(); err != nil {
		cliCtx.Fatalf("tea: %v", err)
	}
}
