package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
		Format string `default:"text" help:"Log format. One of: [text, logfmt, json]"`
	} `embed:"" prefix:"log-"`
	Path    string        `arg:""   help:"File or directory path, default is $PWD."                          optional:"" type:"path"`
	Command []string      `arg:""   help:"Command to run, defaults set in $XDG_CONFIG_HOME/kat/config.yaml." optional:""`
	Config  config.Config `embed:""`
}

func main() {
	configPathExp, err := initializeConfig()
	if err != nil {
		panic(err)
	}

	cliCtx := kong.Parse(&cli,
		kong.Name(cmdName),
		kong.Description(cmdDesc+"\n"+cmdExamples),
		kong.DefaultEnvars(strings.ToUpper(cmdName)),
		kong.Configuration(kongyaml.Loader, configPathExp),
	)

	logHandler, err := log.CreateHandlerWithStrings(cliCtx.Stderr, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.Fatalf("failed to create log handler: %v", err)
	}
	slog.SetDefault(slog.New(logHandler))

	path, err := resolvePath(cli.Path)
	if err != nil {
		slog.Error("resolve paths", slog.Any("err", err))
		cliCtx.Fatalf("initialization failed")
	}
	slog.Debug("parsed args",
		slog.String("path", path),
		slog.Any("command", cli.Command),
	)

	cr, err := setupCommandRunner(path)
	if err != nil {
		slog.Error("setup command runner", slog.Any("err", err))
		cliCtx.Fatalf("initialization failed")
	}

	if err := runUI(cli.Config.UI, cr); err != nil {
		cliCtx.FatalIfErrorf(err)
	}
}

// initializeConfig initializes the configuration file.
func initializeConfig() (string, error) {
	configPathExp := os.ExpandEnv(configPath)
	if err := config.NewConfig().Write(configPathExp); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}
	return configPathExp, nil
}

// resolvePath resolves the input path.
func resolvePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	return absPath, nil
}

// setupCommandRunner creates and configures the command runner
func setupCommandRunner(path string) (*kube.CommandRunner, error) {
	cr := kube.NewCommandRunner(path)

	if len(cli.Command) > 0 {
		cmd := parseCommand(cli.Command)
		cr.SetCommand(cmd)
	} else {
		// No specific command, so use the config file.
		cr.SetCommands(cli.Config.Kube.Commands)
	}

	// Hack: make sure that we can run the command.
	// TODO: implement proper error handling in the UI.
	if _, err := cr.Run(); err != nil {
		return nil, fmt.Errorf("failed to run command: %w", err)
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
func runUI(cfg ui.Config, cr *kube.CommandRunner) error {
	p := ui.NewProgram(cfg, cr)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tea: %w", err)
	}

	return nil
}
