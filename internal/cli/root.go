package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/macropower/kat/pkg/log"
)

const (
	cmdName = "kat"
	cmdDesc = `Rule-based rendering engine and TUI for local Kubernetes manifests.`
)

type RootArgs struct {
	LogLevel  string
	LogFormat string
}

func NewRootArgs() *RootArgs {
	return &RootArgs{}
}

func (ra *RootArgs) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().
		StringVar(&ra.LogLevel, "log-level", "info", fmt.Sprintf("Log level, one of: %s", log.AllLevels))
	cmd.PersistentFlags().
		StringVar(&ra.LogFormat, "log-format", "text", fmt.Sprintf("Log format, one of: %s", log.AllFormats))

	var err error

	err = cmd.RegisterFlagCompletionFunc("log-format",
		cobra.FixedCompletions(log.AllFormats, cobra.ShellCompDirectiveNoFileComp),
	)
	if err != nil {
		panic(err)
	}

	err = cmd.RegisterFlagCompletionFunc("log-level",
		cobra.FixedCompletions(log.AllLevels, cobra.ShellCompDirectiveNoFileComp),
	)
	if err != nil {
		panic(err)
	}
}

func NewRootCmd() *cobra.Command {
	args := NewRootArgs()
	runArgs := NewRunArgs(args)

	runCmd := NewRunCmd(runArgs)
	cmd := &cobra.Command{
		Use:               cmdName,
		Short:             cmdDesc,
		Example:           cmdExamples,
		PersistentPreRunE: setupLogging(args),
		ValidArgsFunction: runCompletion(runArgs),
		Args:              runCmd.Args,
		RunE:              runCmd.RunE,
	}

	args.AddFlags(cmd)
	runArgs.AddFlags(cmd)
	cmd.AddCommand(runCmd)

	bindEnvVars(cmd)

	return cmd
}

func setupLogging(rc *RootArgs) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		logHandler, err := log.CreateHandlerWithStrings(cmd.ErrOrStderr(), rc.LogLevel, rc.LogFormat)
		if err != nil {
			return fmt.Errorf("create log handler: %w", err)
		}

		slog.SetDefault(slog.New(logHandler))

		return nil
	}
}
