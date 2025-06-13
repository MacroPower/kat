package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/alecthomas/kong"

	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/MacroPower/kat/pkg/config"
	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/log"
	"github.com/MacroPower/kat/pkg/ui"
	"github.com/MacroPower/kat/pkg/ui/common"
	uiconfig "github.com/MacroPower/kat/pkg/ui/config"
)

const (
	cmdName     = "kat"
	cmdDesc     = `cat for Kubernetes manifests.`
	cmdInitErr  = "initialization failed"
	cmdExamples = `
Examples:
	# kat the current directory.
	kat .

	# kat a file or directory path.
	kat ./example/kustomize

	# kat with command passthrough.
	kat ./example/kustomize -- kustomize build .

	# kat a file or stdin directly (no reload support).
	cat ./example/kustomize/resources.yaml | kat -f -
`
)

var cli struct {
	Config config.Config `embed:""`

	Log struct {
		Level  string `default:"info" help:"Log level."`
		Format string `default:"text" enum:"text,logfmt,json" help:"Log format."`
	} `embed:"" prefix:"log-"`

	File []byte `env:"-" help:"File content to read." short:"f" type:"filecontent"`

	Path string `arg:"" default:"." help:"File or directory path, default is $PWD." type:"path"`

	Command []string `arg:"" help:"Command to run, defaults set in ~/.config/kat/config.yaml." optional:""`

	Watch       bool `env:"-" help:"Watch for changes and trigger reloading."          short:"w"`
	WriteConfig bool `env:"-" help:"Write the configuration file to the default path."`
	ShowConfig  bool `env:"-" help:"Print the active configuration and exit."`
}

func main() {
	configPath := config.GetPath()

	cliCtx := kong.Parse(&cli,
		kong.Name(cmdName),
		kong.Description(cmdDesc+"\n"+cmdExamples),
		kong.DefaultEnvars(strings.ToUpper(cmdName)),
		kong.Configuration(kongyaml.Loader, configPath),
	)

	logHandler, err := log.CreateHandlerWithStrings(cliCtx.Stderr, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.Fatalf("failed to create log handler: %v", err)
	}
	slog.SetDefault(slog.New(logHandler))

	if cli.WriteConfig {
		if err := config.NewConfig().Write(configPath); err != nil {
			slog.Error("write config", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
	}

	// Load more complex structured configuration that is ignored by kong.
	if err := cli.Config.Load(configPath); err != nil {
		slog.Warn("load config", slog.Any("err", err))
	}
	cli.Config.EnsureDefaults()

	err = cli.Config.UI.KeyBinds.Validate()
	if err != nil {
		slog.Error("validate key binds", slog.Any("err", err))
		cliCtx.Fatalf(cmdInitErr)
	}

	slog.Debug("parsed args",
		slog.String("path", cli.Path),
		slog.Any("command", cli.Command),
	)

	if cli.ShowConfig {
		// Print the active configuration and exit.
		yamlConfig, err := cli.Config.MarshalYAML()
		if err != nil {
			slog.Error("marshal config yaml", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
		slog.Info("active configuration", slog.String("path", configPath))
		fmt.Printf("%s", yamlConfig)
		cliCtx.Exit(0)
	}

	var cr common.Commander

	if len(cli.File) > 0 {
		cr, err = kube.NewResourceGetter(string(cli.File))
		if err != nil {
			slog.Error("create resource getter", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
	} else {
		cr, err = setupCommandRunner(cli.Path)
		if err != nil {
			slog.Error("create command runner", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
	}

	if err := runUI(*cli.Config.UI, cr); err != nil {
		cliCtx.FatalIfErrorf(err)
	}
}

// setupCommandRunner creates and configures the command runner.
func setupCommandRunner(path string) (*kube.CommandRunner, error) {
	var (
		cr  *kube.CommandRunner
		err error
	)

	if len(cli.Command) > 0 {
		cmd := parseCommand(cli.Command)

		cr, err = kube.NewCommandRunner(path, kube.WithCommand(cmd))
		if err != nil {
			return nil, err
		}
	} else {
		cr, err = kube.NewCommandRunner(path, kube.WithCommands(cli.Config.Kube.Commands))
		if err != nil {
			return nil, err
		}
	}

	if cli.Watch {
		err := cr.Watch()
		if err != nil {
			return nil, err
		}
	}

	return cr, nil
}

// parseCommand parses the CLI command arguments into a [kube.Command].
func parseCommand(cmdArgs []string) *kube.Command {
	cmd := &kube.Command{}
	cmdIdx := 0
	if cmdArgs[0] == "--" {
		cmdIdx = 1
	}
	cmd.Command = cmdArgs[cmdIdx]
	if len(cmdArgs) > cmdIdx {
		cmd.Args = cmdArgs[cmdIdx+1:]
	}

	return cmd
}

// runUI starts the UI program.
func runUI(cfg uiconfig.Config, cr common.Commander) error {
	p := ui.NewProgram(cfg, cr)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tea: %w", err)
	}

	return nil
}
