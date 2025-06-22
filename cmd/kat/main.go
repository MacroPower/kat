package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/goccy/go-yaml"

	"github.com/MacroPower/kat/pkg/config"
	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/log"
	"github.com/MacroPower/kat/pkg/ui"
	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/themes"
)

const (
	cmdName     = "kat"
	cmdDesc     = `cat for Kubernetes manifests.`
	cmdInitErr  = "initialization failed"
	cmdExamples = `
Examples:
	# kat the current directory.
	kat

	# kat a file or directory path.
	kat ./example/kustomize

	# Force using the "ks" profile (defined in config).
	kat ./example/kustomize ks

	# Override the "ks" profile arguments.
	kat ./example/kustomize ks -- build . --enable-helm

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

	Command string   `arg:"" help:"Command or profile override."          optional:""`
	Args    []string `arg:"" help:"Arguments for the command or profile." optional:""`

	Compact     bool `env:"-" help:"Enable compact mode for the UI."                   short:"c"`
	Watch       bool `env:"-" help:"Watch for changes and trigger reloading."          short:"w"`
	WriteConfig bool `env:"-" help:"Write the configuration file to the default path."`
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
	configPath := config.GetPath()

	if cli.WriteConfig {
		if err := config.WriteDefaultConfig(configPath); err != nil {
			slog.Error("write config", slog.Any("err", err))
			cliCtx.Fatalf(cmdInitErr)
		}
		slog.Info("configuration written", slog.String("path", configPath))
		cliCtx.Exit(0)
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
		cr, err = kube.NewResourceGetter(string(cli.File))
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

	if err := runUI(cfg, cr); err != nil {
		cliCtx.FatalIfErrorf(fmt.Errorf("ui program failure: %w", err))
	}
}

func getProfile(cfg *config.Config, cmd string, args []string) (*kube.Profile, error) {
	profile, ok := cfg.Kube.Profiles[cmd]
	if !ok {
		// If the command is not a profile, create a new profile with the command.
		slog.Debug("creating new profile", slog.String("name", cmd))
		var err error
		profile, err = kube.NewProfile(cmd, kube.WithArgs(args...))
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
	} else if len(args) > 0 {
		slog.Debug("overwriting profile arguments", slog.String("name", cmd))
		profile.Args = args
	}

	return profile, nil
}

// setupCommandRunner creates and configures the command runner.
func setupCommandRunner(path string, cfg *config.Config) (*kube.CommandRunner, error) {
	var (
		cr  *kube.CommandRunner
		err error
	)

	if cli.Command != "" {
		profile, err := getProfile(cfg, cli.Command, parseArgs(cli.Args))
		if err != nil {
			return nil, err
		}

		cr, err = kube.NewCommandRunner(path, kube.WithProfile(cli.Command, profile))
		if err != nil {
			return nil, err
		}
	} else {
		cr, err = kube.NewCommandRunner(path, kube.WithRules(cfg.Kube.Rules))
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
func runUI(cfg *config.Config, cr common.Commander) error {
	for name, tc := range cfg.UI.Themes {
		err := themes.RegisterTheme(name, tc.Styles)
		if err != nil {
			return fmt.Errorf("theme %q: %w", name, err)
		}
	}

	p := ui.NewProgram(*cfg.UI, cr)

	ch := make(chan kube.CommandEvent)
	cr.Subscribe(ch)

	go func() {
		lastEventTime := time.Now()
		for event := range ch {
			switch e := event.(type) {
			case kube.CommandEventStart:
				p.Send(common.CommandRunStarted{})

			case kube.CommandEventEnd:
				if time.Since(lastEventTime) < *cfg.UI.UI.MinimumDelay {
					// Add a delay if the command ran faster than MinimumDelay.
					// This prevents the status from flickering in the UI.
					time.Sleep(*cfg.UI.UI.MinimumDelay - time.Since(lastEventTime))
				}
				p.Send(common.CommandRunFinished(e))

			case kube.CommandEventCancel:
				continue
			}
			lastEventTime = time.Now()
		}
	}()
	go cr.RunOnEvent()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tea: %w", err)
	}

	return nil
}
