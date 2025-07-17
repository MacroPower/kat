package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/goccy/go-yaml"
	"golang.org/x/term"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/theme"
)

const (
	cmdName     = "kat"
	cmdDesc     = `Rule-based rendering engine and TUI for local Kubernetes manifests.`
	cmdInitErr  = "initialization failed"
	cmdExamples = `
Examples:
	# kat the current directory
	kat

	# kat a file or directory path
	kat ./example/kustomize

	# Force using the "ks" profile (defined in config)
	kat ./example/kustomize ks

	# Override the "ks" profile arguments
	kat ./example/kustomize ks -- build . --enable-helm

	# Watch for changes and reload
	kat ./example/helm --watch

	# kat a file or stdin directly (disables rendering engine)
	cat ./example/kustomize/resources.yaml | kat -f -

	# kat a project and send the output to a file (disables TUI)
	kat ./example/helm > manifests.yaml
`
)

//nolint:revive // CLI.
var cli struct {
	Log struct {
		Level  string `default:"info" help:"Log level."`
		Format string `default:"text" enum:"text,logfmt,json" help:"Log format."`
	} `embed:"" prefix:"log-"`

	File []byte `env:"-" help:"File content to read." short:"f" type:"filecontent"`

	Path string `arg:"" default:"." help:"File or directory path, default is $PWD." type:"path"`

	Config *string `help:"Path to the kat configuration file." optional:"" type:"path"`

	Command string   `arg:"" help:"Command or profile override."          optional:""`
	Args    []string `arg:"" help:"Arguments for the command or profile." optional:""`

	Watch bool `help:"Watch for changes and trigger reloading." short:"w"`

	WriteConfig bool `env:"-" help:"Write the default configuration files."`
	ShowConfig  bool `env:"-" help:"Print the active configuration and exit."`
}

func main() {
	cliCtx := kong.Parse(&cli,
		kong.Name(cmdName),
		kong.Description(cmdDesc+"\n"+cmdExamples),
		kong.DefaultEnvars(strings.ToUpper(cmdName)),
	)

	logHandler, err := log.CreateHandlerWithStrings(cliCtx.Stderr, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.Fatalf("failed to create log handler: %v", err)
	}

	slog.SetDefault(slog.New(logHandler))

	cfg := config.NewConfig()

	var configPath string
	if cli.Config != nil {
		configPath = *cli.Config
	} else {
		configPath = config.GetPath()
	}

	err = config.WriteDefaultConfig(configPath, cli.WriteConfig)
	if err != nil {
		slog.Error("write config", slog.Any("err", err))
	}

	cfgData, err := config.ReadConfig(configPath)
	if err != nil {
		slog.Warn("could not read config, using defaults", slog.Any("err", err))
	} else {
		cfg, err = config.LoadConfig(cfgData)
		if err != nil {
			fmt.Println(yaml.FormatError(err, true, true))

			cliCtx.Fatalf(cmdInitErr)
		}
	}

	err = cfg.UI.KeyBinds.Validate()
	if err != nil {
		slog.Error("validate key binds", slog.Any("err", err))
		cliCtx.Fatalf(cmdInitErr)
	}

	slog.Debug("parsed args",
		slog.String("path", cli.Path),
		slog.Any("command", cli.Command),
		slog.Any("args", cli.Args),
	)

	if cli.ShowConfig {
		// Print the active configuration and exit.
		yamlConfig, err := cfg.MarshalYAML()
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
		cr, err = command.NewStatic(string(cli.File))
		if err != nil {
			slog.Error("create resource getter", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
	} else {
		cr, err = setupCommandRunner(cli.Path, cfg)
		if err != nil {
			slog.Error("create command runner", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
	}

	// If stdout is not a terminal, actually "concatenate".
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		run := cr.Run()
		if run.Stdout != "" {
			_, err := fmt.Fprint(cliCtx.Stdout, run.Stdout)
			cliCtx.FatalIfErrorf(err)
		}
		if run.Stderr != "" {
			_, err := fmt.Fprint(cliCtx.Stderr, run.Stderr)
			cliCtx.FatalIfErrorf(err)
		}

		cliCtx.FatalIfErrorf(run.Error)

		// Exit early.
		cliCtx.Exit(0)
	}

	logBuf := log.NewCircularBuffer(100)
	logHandler, err = log.CreateHandlerWithStrings(logBuf, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.Fatalf("failed to create log handler: %v", err)
	}

	slog.SetDefault(slog.New(logHandler))

	err = runUI(cfg.UI, cr)
	if err != nil {
		slog.Error("run UI", slog.Any("err", err))
		flushLogs(cliCtx.Stderr, logBuf)
		cliCtx.FatalIfErrorf(fmt.Errorf("ui program failure: %w", err))
	}

	flushLogs(cliCtx.Stderr, logBuf)
}

func flushLogs(w io.Writer, buf *log.CircularBuffer) {
	slog.Debug("flush logs to console",
		slog.Int("count", buf.Size()),
		slog.Int("max", buf.Capacity()),
		slog.Bool("truncated", buf.IsFull()),
	)

	_, err := buf.WriteTo(w)
	if err != nil {
		panic(err)
	}
}

func getProfile(cfg *config.Config, cmd string, args []string) (*profile.Profile, error) {
	p, ok := cfg.Command.Profiles[cmd]
	if !ok {
		// If the command is not a profile, create a new profile with the command.
		slog.Debug("creating new profile", slog.String("name", cmd))

		var err error

		p, err = profile.New(cmd, profile.WithArgs(args...))
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
	} else if len(args) > 0 {
		slog.Debug("overwriting profile arguments", slog.String("name", cmd))

		p.Command.Args = args
	}

	return p, nil
}

// setupCommandRunner creates and configures the command runner.
func setupCommandRunner(path string, cfg *config.Config) (*command.Runner, error) {
	var (
		cr  *command.Runner
		err error
	)

	if cli.Command != "" {
		p, err := getProfile(cfg, cli.Command, parseArgs(cli.Args))
		if err != nil {
			return nil, err
		}

		cr, err = command.NewRunner(path, command.WithProfile(cli.Command, p))
		if err != nil {
			return nil, err
		}
	} else {
		cr, err = command.NewRunner(path, command.WithRules(cfg.Command.Rules))
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

func parseArgs(cmdArgs []string) []string {
	if len(cmdArgs) == 0 {
		return []string{}
	}

	argIdx := 0
	if cmdArgs[0] == "--" {
		argIdx = 1
	}

	args := cmdArgs[argIdx:]

	return args
}

// runUI starts the UI program.
func runUI(cfg *ui.Config, cr common.Commander) error {
	for name, tc := range cfg.Themes {
		err := theme.Register(name, tc.Styles)
		if err != nil {
			return fmt.Errorf("theme %q: %w", name, err)
		}
	}

	p := ui.NewProgram(cfg, cr)

	ch := make(chan command.Event)
	cr.Subscribe(ch)

	go func() {
		lastEventTime := time.Now()
		for event := range ch {
			switch e := event.(type) {
			case command.EventStart:
				p.Send(e)

			case command.EventEnd:
				if time.Since(lastEventTime) < *cfg.UI.MinimumDelay {
					// Add a delay if the command ran faster than MinimumDelay.
					// This prevents the status from flickering in the UI.
					time.Sleep(*cfg.UI.MinimumDelay - time.Since(lastEventTime))
				}

				p.Send(e)

			case command.EventCancel:
				continue
			}

			lastEventTime = time.Now()
		}
	}()
	go cr.RunOnEvent()

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("tea: %w", err)
	}

	return nil
}
