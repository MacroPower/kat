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
	cmdName     = "kat"
	cmdDesc     = `cat for Kubernetes manifests.`
	cmdExamples = `
Examples:
	# kat the current directory.
	kat .

	# kat a file or directory path.
	kat ./example/kustomize

	# kat with command passthrough.
	kat ./example/kustomize -- kustomize build .
`

	configPath = "$XDG_CONFIG_HOME/kat/config.yaml"
)

var cli struct {
	Log struct {
		Level  string `default:"info"   help:"Log level."`
		Format string `default:"logfmt" help:"Log format. One of: [logfmt, json]"`
	} `embed:"" prefix:"log-"`
	Path    string        `arg:"" help:"File or directory path, default is $PWD." required:"" type:"path"`
	Command []string      `arg:"" help:"Command to run, defaults set in $XDG_CONFIG_HOME/kat/config.yaml." optional:"" passthrough:"all"`
	Config  config.Config `embed:""`
}

func main() {
	configPathExp := os.ExpandEnv(configPath)
	err := config.NewConfig().Write(configPathExp)

	cliCtx := kong.Parse(&cli,
		kong.Name(cmdName),
		kong.Description(cmdDesc+"\n"+cmdExamples),
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

	cr := kube.NewCommandRunner(path)
	if len(cli.Command) > 0 {
		cmd := &kube.Command{}
		cmdIdx := 0
		if cli.Command[0] == "--" {
			cmdIdx = 1
		}
		cmd.Command = cli.Command[cmdIdx]
		if len(cli.Command) > cmdIdx {
			cmd.Args = cli.Command[cmdIdx+1:]
		}
		cr.SetCommand(cmd)
	} else {
		// No specific command, so use the config file.
		cr.SetCommands(cli.Config.Kube.Commands)
	}

	// Hack: make sure that we can run the command.
	// TODO: implement proper error handling in the UI.
	if _, err = cr.Run(); err != nil {
		cliCtx.FatalIfErrorf(err)
	}

	p := ui.NewProgram(cli.Config.UI, cr)
	if _, err := p.Run(); err != nil {
		cliCtx.Fatalf("tea: %v", err)
	}
}
