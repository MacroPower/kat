package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/alecthomas/kong"
	"sigs.k8s.io/yaml"

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
	Log struct {
		Level  string `default:"info" help:"Log level."`
		Format string `default:"text" enum:"text,logfmt,json" help:"Log format."`
	} `embed:"" prefix:"log-"`

	File []byte `env:"-" help:"File content to read." short:"f" type:"filecontent"`

	Path string `arg:"" default:"." help:"File or directory path, default is $PWD." type:"path"`

	Command []string `arg:"" help:"Command to run, defaults set in ~/.config/kat/config.yaml." optional:""`

	Config config.Config `embed:""`

	ShowConfig bool `env:"-" help:"Print the active configuration and exit."`
}

func main() {
	configPath, err := initializeConfig()
	if err != nil {
		panic(err)
	}

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

	// Load more complex structured configuration that is ignored by kong.
	if err := cli.Config.Load(configPath); err != nil {
		slog.Error("load config", slog.Any("err", err))
		cliCtx.Fatalf("initialization failed")
	}

	if cli.Config.UI.KeyBinds == nil {
		slog.Debug("using default key binds")
		cli.Config.UI.KeyBinds = uiconfig.NewKeyBinds()
	} else {
		cli.Config.UI.KeyBinds.EnsureDefaults()
	}

	err = cli.Config.UI.KeyBinds.Validate()
	if err != nil {
		slog.Error("validate key binds", slog.Any("err", err))
		cliCtx.Fatalf("initialization failed")
	}

	slog.Debug("parsed args",
		slog.String("path", cli.Path),
		slog.Any("command", cli.Command),
	)

	if cli.ShowConfig {
		// Print the active configuration and exit.
		yamlConfig, err := yaml.Marshal(cli.Config)
		if err != nil {
			slog.Error("marshal config yaml", slog.Any("err", err))
			cliCtx.Fatalf("initialization failed")
		}
		cliCtx.Printf("active configuration: %s\n", configPath)
		fmt.Printf("%s", yamlConfig)
		cliCtx.Exit(0)
	}

	var cr common.Commander

	if len(cli.File) > 0 {
		cr, err = kube.NewResourceGetter(string(cli.File))
		if err != nil {
			slog.Error("create resource getter", slog.Any("err", err))
			cliCtx.Fatalf("initialization failed")
		}
	} else {
		cr = setupCommandRunner(cli.Path)
	}

	if err := runUI(cli.Config.UI, cr); err != nil {
		cliCtx.FatalIfErrorf(err)
	}
}

// initializeConfig initializes the configuration file.
func initializeConfig() (string, error) {
	configPath := config.GetPath()
	if err := config.NewConfig().Write(configPath); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, nil
}

// setupCommandRunner creates and configures the command runner.
func setupCommandRunner(path string) *kube.CommandRunner {
	cr := kube.NewCommandRunner(path)

	if len(cli.Command) > 0 {
		cmd := parseCommand(cli.Command)
		cr.SetCommand(cmd)
	} else {
		// No specific command, so use the config file.
		cr.SetCommands(cli.Config.Kube.Commands)
	}

	return cr
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
