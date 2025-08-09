package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/mcp"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const (
	cmdExamples = `  # kat the current directory:
  kat

  # kat a file or directory path:
  kat ./example/kustomize

  # Watch for changes and reload:
  kat ./example/helm --watch

  # Force using the "ks" profile (defined in config):
  kat ./example/kustomize ks

  # Set the extra arguments:
  kat ./example/helm -- -g -f prod-values.yaml

  # Read from stdin (disables rendering engine):
  cat ./example/kustomize/resources.yaml | kat -

  # Send output to a file (disables TUI):
  kat ./example/helm > manifests.yaml`
)

type RunArgs struct {
	*RootArgs

	Path             string
	StdinData        []byte
	ConfigPath       string
	CommandOrProfile string
	Args             []string
	ServeMCP         string
	Watch            bool
	WriteConfig      bool
	ShowConfig       bool
}

func NewRunArgs(rootArgs *RootArgs) *RunArgs {
	return &RunArgs{
		RootArgs: rootArgs,
	}
}

func (ra *RunArgs) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&ra.ConfigPath, "config", "", "Path to the kat configuration file")
	cmd.Flags().StringVar(&ra.ServeMCP, "serve-mcp", "", "Serve the MCP server at the specified address")
	cmd.Flags().BoolVarP(&ra.Watch, "watch", "w", false, "Watch for changes and trigger reloading")
	cmd.Flags().BoolVar(&ra.WriteConfig, "write-config", false, "Write the default configuration files and exit")
	cmd.Flags().BoolVar(&ra.ShowConfig, "show-config", false, "Print the active configuration and exit")

	err := cmd.MarkFlagFilename("config", "yaml", "yml")
	if err != nil {
		panic(fmt.Errorf("mark config flag: %w", err))
	}
}

func NewRunCmd(ra *RunArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run [path] [profile]",
		Short:   "Default command, can be used explicitly if path/command is ambiguous",
		Example: cmdExamples,
		Args: func(cmd *cobra.Command, args []string) error {
			// Check if we have more than 2 args before the dash.
			dashPos := cmd.ArgsLenAtDash()
			if dashPos == -1 {
				// No dash, so all args count.
				if len(args) > 2 {
					return fmt.Errorf("accepts at most 2 args, received %d", len(args))
				}
			} else if dashPos > 2 {
				// Too many args before the dash.
				return fmt.Errorf("accepts at most 2 args before --, received %d", dashPos)
			}
			return nil
		},
		ValidArgsFunction: runCompletion(ra),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				path             = "."
				commandOrProfile string
				extraArgs        []string
			)

			// Handle args before the dash (or all args if no dash).
			dashPos := cmd.ArgsLenAtDash()
			argsBeforeDash := args
			if dashPos != -1 {
				argsBeforeDash = args[:dashPos]
				extraArgs = args[dashPos:]
			}
			if len(argsBeforeDash) > 0 {
				path = argsBeforeDash[0]
			}
			if len(argsBeforeDash) > 1 {
				commandOrProfile = argsBeforeDash[1]
			}

			ra.Path = path
			ra.CommandOrProfile = commandOrProfile
			ra.Args = extraArgs

			return run(cmd, ra)
		},
	}
	ra.AddFlags(cmd)

	bindEnvVars(cmd)

	return cmd
}

// Try to load config to get available profiles.
func tryGetProfileNames(configPath string) []cobra.Completion {
	if configPath == "" {
		configPath = config.GetPath()
	}

	cl, err := config.NewConfigLoaderFromFile(configPath)
	if err != nil {
		return nil
	}

	cfg, err := cl.Load()
	if err != nil {
		return nil
	}

	profileNameDesc := map[string]string{}
	for k, v := range cfg.Command.Profiles {
		profileNameDesc[k] = v.String()
	}
	if len(profileNameDesc) == 0 {
		return nil
	}

	completions := make([]cobra.Completion, 0, len(profileNameDesc))
	for name, desc := range profileNameDesc {
		completions = append(completions, cobra.CompletionWithDesc(name, desc))
	}

	return completions
}

func runCompletion(ra *RunArgs) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		// First argument: path completion.
		if len(args) == 0 {
			return nil, cobra.ShellCompDirectiveFilterDirs
		}

		// Dash argument: extra args completion.
		// This needs to happen after the first argument is handled, even though
		// passing the dash argument first still works. This is to prevent showing
		// subcommands as completions for the first argument.
		dashPos := argsLenAtDash(os.Args)
		if dashPos != -1 && len(args) >= dashPos {
			// TODO: Try to complete the extra args based on the command or profile.
			return nil, cobra.ShellCompDirectiveDefault
		}

		// Second argument: command/profile completion.
		if len(args) == 1 {
			return tryGetProfileNames(ra.ConfigPath), cobra.ShellCompDirectiveNoFileComp
		}

		// No more arguments accepted.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// Hack to find the position of the first dash argument.
// Can be removed after https://github.com/spf13/cobra/pull/2259 is merged.
func argsLenAtDash(args []string) int {
	var dashPos int
	for _, arg := range args {
		if arg == "__complete" {
			// Ignore the __complete argument.
			continue
		}

		if arg == "--" {
			return dashPos - 1
		}

		dashPos++
	}

	return -1 // No dash argument found.
}

func run(cmd *cobra.Command, rc *RunArgs) error {
	if rc.Path == "-" {
		in := cmd.InOrStdin()
		b, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}

		rc.StdinData = b
	}

	cfg := config.NewConfig()

	var configPath string
	if rc.ConfigPath != "" {
		configPath = rc.ConfigPath
	} else {
		configPath = config.GetPath()
	}

	err := config.WriteDefaultConfig(configPath, false)
	if err != nil {
		slog.Error("write default config", slog.Any("err", err))
	}
	if rc.WriteConfig {
		// Exit early after writing the default config.
		// Also, if there was an error, it should be fatal.
		return err
	}

	cl, err := config.NewConfigLoaderFromFile(configPath, config.WithThemeFromData())
	if err != nil {
		slog.Warn("could not read config, using defaults", slog.Any("err", err))
	} else {
		err = cl.Validate()
		if err != nil {
			return fmt.Errorf("invalid config %q: %w", configPath, err)
		}

		cfg, err = cl.Load()
		if err != nil {
			return fmt.Errorf("invalid config %q: %w", configPath, err)
		}
	}

	err = cfg.UI.KeyBinds.Validate()
	if err != nil {
		return fmt.Errorf("validate key binds: %w", err)
	}

	if rc.ShowConfig {
		// Print the active configuration and exit.
		slog.Info("active configuration", slog.String("path", configPath))

		yamlBytes, err := cfg.MarshalYAML()
		if err != nil {
			return fmt.Errorf("marshal config yaml: %w", err)
		}

		yamlConfig := string(yamlBytes)

		cr := yamls.NewChromaRenderer(cl.GetTheme(), yamls.WithLineNumbersDisabled(true))
		prettyConfig, err := cr.RenderContent(yamlConfig, 0)
		if err != nil {
			mustN(fmt.Fprintln(cmd.OutOrStdout(), yamlConfig))

			return err
		}

		mustN(fmt.Fprintln(cmd.OutOrStdout(), prettyConfig))

		return nil
	}

	var cr common.Commander

	if len(rc.StdinData) > 0 {
		cr, err = command.NewStatic(string(rc.StdinData))
		if err != nil {
			return fmt.Errorf("create resource getter: %w", err)
		}
	} else {
		cr, err = setupCommandRunner(rc.Path, cfg, rc)
		if err != nil {
			return fmt.Errorf("create command runner: %w", err)
		}
	}

	// If stdout is not a terminal, actually "concatenate".
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		run := cr.Run()
		err := writeToOutput(cmd, run)
		if err != nil {
			return err
		}

		// Exit early.
		return nil
	}

	logBuf := log.NewCircularBuffer(100)
	logHandler, err := log.CreateHandlerWithStrings(logBuf, rc.LogLevel, rc.LogFormat)
	if err != nil {
		return fmt.Errorf("create log handler: %w", err)
	}

	slog.SetDefault(slog.New(logHandler))

	if rc.ServeMCP != "" {
		mcpServer, err := mcp.NewServer(rc.ServeMCP, cr)
		if err != nil {
			return fmt.Errorf("create MCP server: %w", err)
		}

		go func() {
			err := mcpServer.Serve(context.Background())
			if err != nil {
				slog.Error("MCP server failed", slog.Any("err", err))
			}
		}()
	}

	err = runUI(cfg.UI, cr)
	if err != nil {
		slog.Error("run UI", slog.Any("err", err))
		flushLogs(cmd.ErrOrStderr(), logBuf)

		return fmt.Errorf("ui program failure: %w", err)
	}

	flushLogs(cmd.ErrOrStderr(), logBuf)

	return nil
}

func writeToOutput(cmd *cobra.Command, run command.Output) error {
	if run.Stdout != "" {
		_, err := fmt.Fprint(cmd.OutOrStdout(), run.Stdout)
		if err != nil {
			return fmt.Errorf("write to stdout: %w", err)
		}
	}
	if run.Stderr != "" {
		_, err := fmt.Fprint(cmd.ErrOrStderr(), run.Stderr)
		if err != nil {
			return fmt.Errorf("write to stderr: %w", err)
		}
	}
	if run.Error != nil {
		return fmt.Errorf("run error: %w", run.Error)
	}

	return nil
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

		p, err = profile.New(cmd, profile.WithExtraArgs(args...))
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
	}

	return p, nil
}

// setupCommandRunner creates and configures the command runner.
func setupCommandRunner(path string, cfg *config.Config, rc *RunArgs) (*command.Runner, error) {
	var (
		cr  *command.Runner
		err error
	)

	if rc.CommandOrProfile != "" {
		p, err := getProfile(cfg, rc.CommandOrProfile, rc.Args)
		if err != nil {
			return nil, err
		}

		cr, err = command.NewRunner(path, command.WithCustomProfile(rc.CommandOrProfile, p))
		if err != nil {
			return nil, err
		}
	} else {
		cr, err = command.NewRunner(path,
			command.WithRules(cfg.Command.Rules),
			command.WithProfiles(cfg.Command.Profiles),
			command.WithExtraArgs(rc.Args...),
			command.WithWatch(rc.Watch),
		)
		if err != nil {
			return nil, err
		}
	}

	return cr, nil
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

			case command.EventConfigure:
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
