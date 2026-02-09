package cli

import (
	"log/slog"

	"github.com/spf13/cobra"

	xlog "go.jacobcolvin.com/x/log"
)

const (
	cmdName = "kat"
	cmdDesc = `Rule-based rendering engine and TUI for local Kubernetes manifests.`
)

type RootArgs struct {
	Log *xlog.Config
}

func NewRootArgs() *RootArgs {
	return &RootArgs{}
}

func (ra *RootArgs) AddFlags(cmd *cobra.Command) {
	ra.Log = xlog.NewConfig()
	ra.Log.RegisterFlags(cmd.PersistentFlags())

	err := ra.Log.RegisterCompletions(cmd)
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
		logHandler, err := rc.Log.NewHandler(cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		slog.SetDefault(slog.New(logHandler))

		return nil
	}
}
